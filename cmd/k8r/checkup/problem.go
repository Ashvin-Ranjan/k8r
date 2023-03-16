// Taken from: https://github.com/getoutreach/devenv/blob/main/cmd/devenv/debug/problem.go
// Rights given from Outreach under Apache License 2.0

// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains code for problem creation

package checkup

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// Service is a the severity of a problem
type Severity int

// serverity levels
const (
	// SeverityError is an error
	SeverityError Severity = iota
	// SeverityWarning is a warning
	SeverityWarning
)

// Problem is a problem that was found in the devenv environment
// EDIT: Change Detector method signature
type Problem struct {
	// ID is a unique identifier for the problem used to group
	// problems together for different resources.
	ID string

	// ShortDescription is a short description of the problem
	ShortDescription string

	// HelpURL is a URL that can be used to help the user resolve
	// the problem. Defaults to the devenv wki/ID.
	HelpURL string

	// Detector is a function that detects if this problem exists.
	Detector func(context.Context, runtime.Object, *Config) (resourceSpecificReason string, warning, isOccurring bool)
}

// Resource is a resource that has a problem associated with it
type Resource struct {
	// Name is the name of the resource having a problem,
	// this is usually a pod name or the like.
	Name string

	// Owner is the team that owns this resource, if that information
	// is present.
	Owner string

	// Type is the type of resource that is having a problem
	// e.g. pod, deployment, etc.
	Type string

	// ProblemID is the ID of the problem that is occurring
	ProblemID string

	// ProblemDetails is details about the problem specific
	// to the resource.
	ProblemDetails string

	// Warning denotes if this is a warning or not, e.g. isn't actually
	// causing a problem _now_. This is usually used for problems that
	// previously occurred or aren't otherwise currently occurring.
	Warning bool
}

// Report is a report of problems that were found in
// the devenv environment
type Report struct {
	// Problems is a list of problems that were found
	Problems []Problem

	// Resources is a list of resources that were found
	// that had a given problem
	Resources []Resource
}

// GetProblemByID returns a problem by ID
func (r *Report) GetProblemByID(id string) *Problem {
	for i := range r.Problems {
		problem := &r.Problems[i]
		if problem.ID == id {
			return problem
		}
	}
	return nil
}

// ByProblem returns a map of resources by problem ID
func (r *Report) ByProblem() map[string][]*Resource {
	rtrn := make(map[string][]*Resource)

	for i := range r.Problems {
		problem := &r.Problems[i]
		for j := range r.Resources {
			resource := &r.Resources[j]
			if resource.ProblemID == problem.ID {
				rtrn[problem.ID] = append(rtrn[problem.ID], resource)
			}
		}
	}

	return rtrn
}

// BySeverity returns a map of problems by severity with a map of
// resources by problem ID
func (r *Report) BySeverity() map[Severity]map[string][]*Resource {
	rtrn := make(map[Severity]map[string][]*Resource)

	// initialize the map
	rtrn[SeverityError] = make(map[string][]*Resource)
	rtrn[SeverityWarning] = make(map[string][]*Resource)

	for i := range r.Problems {
		problem := &r.Problems[i]
		for j := range r.Resources {
			resource := &r.Resources[j]
			if resource.ProblemID == problem.ID {
				if !resource.Warning {
					rtrn[SeverityError][problem.ID] = append(rtrn[SeverityError][problem.ID], resource)
				} else {
					rtrn[SeverityWarning][problem.ID] = append(rtrn[SeverityWarning][problem.ID], resource)
				}
			}
		}
	}

	return rtrn
}

// ReportFromResources creates a report from a list of resources
func ReportFromResources(resources []Resource) Report {
	problemHM := make(map[string]struct{})
	report := Report{
		Problems:  make([]Problem, 0),
		Resources: resources,
	}

	for _, resource := range resources {
		// Only add the problem once
		if _, ok := problemHM[resource.ProblemID]; ok {
			continue
		}

		for _, enabled := range enabledProblems {
			if enabled.ID == resource.ProblemID {
				report.Problems = append(report.Problems, enabled)
				problemHM[enabled.ID] = struct{}{}
				break
			}
		}
	}

	return report
}
