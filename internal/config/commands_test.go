package config

import "testing"

func TestMergeCommandsAddsMissing(t *testing.T) {
	defaults := CommandsConfig{
		"copy": {Description: "Copy", Action: ActionConfig{Name: "copy"}},
		"new":  {Description: "New", Action: ActionConfig{Name: "new"}},
	}
	user := CommandsConfig{
		"copy": {Description: "Моя копия", Action: ActionConfig{Name: "copy"}},
	}

	merged, changed := mergeCommands(defaults, user)
	if !changed {
		t.Fatal("ожидалось изменение: новой команды не было")
	}
	if merged["copy"].Description != "Моя копия" {
		t.Fatal("пользовательская команда была перезаписана")
	}
	if _, ok := merged["new"]; !ok {
		t.Fatal("новая встроенная команда не добавлена")
	}
}

func TestMergeCommandsNoChanges(t *testing.T) {
	defaults := CommandsConfig{"copy": {Description: "Copy"}}
	user := CommandsConfig{"copy": {Description: "Copy"}}
	_, changed := mergeCommands(defaults, user)
	if changed {
		t.Fatal("изменений быть не должно")
	}
}
