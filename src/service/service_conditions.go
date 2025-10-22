package service

import (
	"fmt"
	"log"
	"os"
)

type ServiceCondition struct {
	Hostname *string `json:"hostname"`
}

func EvaluateConditions(conditions []ServiceCondition, verbose bool) (res bool, err error) {
	if len(conditions) == 0 {
		return true, nil
	}

	for _, cond := range conditions {
		res, err = cond.Evaluate(verbose)
		if res || err != nil {
			return res, err
		}
	}

	return false, nil
}

func (cond *ServiceCondition) Evaluate(verbose bool) (bool, error) {
	var result = false

	if cond.Hostname != nil {
		hostname, err := os.Hostname()
		if err != nil {
			return false, fmt.Errorf("could not check hostname condition for %+s, %v", *cond.Hostname, err)
		}

		result = hostname == *cond.Hostname
		if verbose {
			log.Printf("Check service condition hostname=%+v: %v", *cond.Hostname, result)
		}
		if !result {
			return false, nil
		}
	}

	return result, nil
}
