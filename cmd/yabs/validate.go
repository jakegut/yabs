package main

import (
	"fmt"

	"github.com/risor-io/risor/object"
)

func validateString(obj object.Object) (string, error) {
	strObj, ok := obj.(*object.String)
	if !ok {
		return "", fmt.Errorf("expected string, got=%T", obj)
	}
	return strObj.String(), nil
}

type ValidateListOf interface {
	~string
}

func validateList[T ValidateListOf](obj object.Object) ([]T, error) {
	listObj, ok := obj.(*object.List)
	if !ok {
		return make([]T, 0), fmt.Errorf("expected list, got=%T", obj)
	}
	listOf := []T{}
	it := listObj.Iter()
	t := *new(T)
	for {
		dep, ok := it.Next()
		if !ok {
			break
		}
		switch any(t).(type) {
		case string:
			strObj, ok := dep.(*object.String)
			if !ok {
				return make([]T, 0), fmt.Errorf("wrong type of list, expected got=%T, want %T", dep, t)
			}
			listOf = append(listOf, any(strObj.String()).(T))
		}
	}
	return listOf, nil
}