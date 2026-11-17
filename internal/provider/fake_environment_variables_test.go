package provider

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
)

// The fake API keeps environment variables in memory, so tests can set and
// delete them like the live API does: POST /environments/{id}/variables and
// POST /environments/{id}/variables/delete change them, and GET
// /environments/{id} returns them. Tests change them "outside Terraform" with
// setVariable and deleteVariable.

type fakeVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var (
	environmentPath          = regexp.MustCompile(`^/environments/([^/]+)$`)
	environmentVariablesPath = regexp.MustCompile(`^/environments/([^/]+)/variables(/delete)?$`)
	variableKeyPattern       = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// serveEnvironmentVariables handles the requests that change variables, and
// reports whether it handled r.
func (a *fakeAPI) serveEnvironmentVariables(w http.ResponseWriter, r *http.Request) bool {
	match := environmentVariablesPath.FindStringSubmatch(r.URL.Path)
	if match == nil || r.Method != http.MethodPost {
		return false
	}
	environment, isDelete := match[1], match[2] != ""

	a.mu.Lock()
	defer a.mu.Unlock()
	variables, ok := a.variablesLocked(environment)
	if !ok {
		http.Error(w, `{"message":"Not found."}`, http.StatusNotFound)
		return true
	}

	if isDelete {
		var body struct {
			Keys []string `json:"keys"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Keys) == 0 {
			unprocessable(w, "The keys field is required.")
			return true
		}
		// Like the live API, the delete is atomic: one missing key fails it.
		for _, key := range body.Keys {
			if !hasVariable(variables, key) {
				unprocessable(w, "The following keys do not exist: "+key)
				return true
			}
		}
		remaining := variables[:0:0]
		for _, v := range variables {
			if !slices.Contains(body.Keys, v.Key) {
				remaining = append(remaining, v)
			}
		}
		a.variables[environment] = remaining
	} else {
		var body struct {
			Method    string         `json:"method"`
			Variables []fakeVariable `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Variables) == 0 {
			unprocessable(w, "The variables field is required.")
			return true
		}
		if body.Method != "set" && body.Method != "append" {
			unprocessable(w, "The selected method is invalid.")
			return true
		}
		for _, v := range body.Variables {
			if !variableKeyPattern.MatchString(v.Key) {
				unprocessable(w, "The key format is invalid.")
				return true
			}
			a.variables[environment] = putVariable(a.variables[environment], v, body.Method == "set")
		}
	}

	payload, _ := readRecording("/environments/" + environment)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(replaceVariables(payload, a.variables[environment]))
	return true
}

// withEnvironmentVariables returns an environment's recorded response with the
// variables as they are now.
func (a *fakeAPI) withEnvironmentVariables(requestPath string, payload []byte) []byte {
	match := environmentPath.FindStringSubmatch(requestPath)
	if match == nil {
		return payload
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	variables, ok := a.variablesLocked(match[1])
	if !ok {
		return payload
	}
	return replaceVariables(payload, variables)
}

// variablesLocked returns an environment's variables, seeding them from the
// recorded environment the first time.
func (a *fakeAPI) variablesLocked(environment string) ([]fakeVariable, bool) {
	if variables, ok := a.variables[environment]; ok {
		return variables, true
	}
	payload, err := readRecording("/environments/" + environment)
	if err != nil {
		return nil, false
	}
	var document struct {
		Data struct {
			Attributes struct {
				EnvironmentVariables []fakeVariable `json:"environment_variables"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, false
	}
	if a.variables == nil {
		a.variables = map[string][]fakeVariable{}
	}
	a.variables[environment] = document.Data.Attributes.EnvironmentVariables
	return a.variables[environment], true
}

// setVariable sets a variable outside Terraform.
func (a *fakeAPI) setVariable(environment, key, value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.variablesLocked(environment)
	a.variables[environment] = putVariable(a.variables[environment], fakeVariable{Key: key, Value: value}, true)
}

// deleteVariable deletes a variable outside Terraform.
func (a *fakeAPI) deleteVariable(environment, key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	variables, _ := a.variablesLocked(environment)
	remaining := variables[:0:0]
	for _, v := range variables {
		if v.Key != key {
			remaining = append(remaining, v)
		}
	}
	a.variables[environment] = remaining
}

// variable returns the value of an environment's variable.
func (a *fakeAPI) variable(environment, key string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	variables, _ := a.variablesLocked(environment)
	for _, v := range variables {
		if v.Key == key {
			return v.Value, true
		}
	}
	return "", false
}

// putVariable sets v in variables. With replace, the "set" method, it
// replaces the value of the same key; otherwise, the "append" method, it adds
// v even when the key exists.
func putVariable(variables []fakeVariable, v fakeVariable, replace bool) []fakeVariable {
	updated := append([]fakeVariable(nil), variables...)
	if replace {
		found := false
		for i := range updated {
			if updated[i].Key == v.Key {
				updated[i].Value = v.Value
				found = true
			}
		}
		if found {
			return updated
		}
	}
	return append(updated, v)
}

func hasVariable(variables []fakeVariable, key string) bool {
	for _, v := range variables {
		if v.Key == key {
			return true
		}
	}
	return false
}

// replaceVariables sets data.attributes.environment_variables in a recorded
// environment document.
func replaceVariables(payload []byte, variables []fakeVariable) []byte {
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		return payload
	}
	data, _ := document["data"].(map[string]any)
	attributes, _ := data["attributes"].(map[string]any)
	if attributes == nil {
		return payload
	}
	if variables == nil {
		variables = []fakeVariable{}
	}
	attributes["environment_variables"] = variables
	updated, err := json.Marshal(document)
	if err != nil {
		return payload
	}
	return updated
}

func unprocessable(w http.ResponseWriter, message string) {
	encoded, _ := json.Marshal(map[string]string{"message": message})
	http.Error(w, string(encoded), http.StatusUnprocessableEntity)
}
