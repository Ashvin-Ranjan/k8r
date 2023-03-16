// Taken from: https://github.com/getoutreach/devenv/blob/main/cmd/devenv/debug/problem_pods.go
// Rights given from Outreach under Apache License 2.0
// All will be demarkated with comments

// Copyright 2022 Outreach Corporation. All Rights Reserved.

// EDITED DESCRIPTION: Use 'k8r checkup' instead of 'devenv debug'
// Description: This file contains the code for the 'k8r cehckup'
// command.

// EDITED DESCRIPTION: Descripe checkup instead of debug
// Package checkup implements a 'k8r checkup' command that allows
// developers to debug their Kubernetes clusters.
package checkup

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/getoutreach/devenv/pkg/kube"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enabledPodProblems is a list of pod problem checkers that are enabled
var enabledPodProblems = []Problem{
	ProblemPodCrashLoopBackOff,
	ProblemPodNotReady,
	ProblemPodImagePullBackOff,
	ProblemPodOOMKilled,
	// EDITS: New problems added
	ProblemHighRestarts,
}

// EDIT: 2 new lists added
// enabledHPAProblems is a list of HPA problem checkers that are enabled
var enabledHPAProblems = []Problem{
	ProblemMaxedOutHPAs,
}

// enbaledProblems is a list of all problem checkers that are enabled
var enabledProblems = append(enabledPodProblems, enabledHPAProblems...)

// contains string helpers
var (
	// bold returns a string in bold
	bold      = color.New(color.Bold)
	underline = color.New(color.Underline)
)

// Options contains options for the devenv debug
// command
// EDIT: Change fields to add in Config
type Options struct {
	log logrus.FieldLogger
	cfg *Config
}

// NewOptions contains options for the devenv debug
// command
func NewOptions(log logrus.FieldLogger) *Options {
	return &Options{
		log: log,
	}
}

// NewCommand creates a new devenv debug command
func NewCommand(log logrus.FieldLogger) *cli.Command {
	o := NewOptions(log)

	return &cli.Command{
		// Edited Name and Usage of command
		Name:  "checkup",
		Usage: "Debug Kubernetes clusters",
		// EDIT: Pass in config
		Action: func(c *cli.Context) error {
			o.cfg = &Config{
				RestartThreshold: c.Int("restart-threshold"),
			}
			return o.Run(c.Context)
		},
		// EDIT: Add flags
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "restart-threshold",
				Usage: "Sets the restart threshold for the HighRestarts problem",
				Value: 3,
			},
		},
	}
}

// EDIT: Add type
// Config stored all the flags passed in
type Config struct {
	// RestartThreshold is from the restart-threshold flag
	RestartThreshold int
}

// ResourceProblem is a problem with a resource, e.g. a pod
type ResourceProblem struct {
	// Owner is the team that owns this resource, if that information
	// is present.
	ResourceOwner string

	// ResourceName is the name of the resource having a problem,
	// this is usually a pod name or the like.
	ResourceName string

	// ResourceType is the type of resource that is having a problem
	// e.g. pod, deployment, etc.
	ResourceType string

	// ResourceProblemDetails is details about the problem specific
	// to the resource.
	ResourceProblemDetails string

	// Warning denotes if this is a warning or not, e.g. isn't actually
	// causing a problem _now_. This is usually used for problems that
	// previously occurred or aren't otherwise currently occurring.
	Warning bool

	// Problem is the problem that is happening with the resource
	Problem Problem
}

// getPodsWithProblems creates a list of problems i/r/t pods
func (o *Options) getPodsWithProblems(ctx context.Context, pod *corev1.Pod) ([]Resource, bool) {
	problems := make([]Resource, 0)

	// defaultProblem is a problem that for the pod with prefilled
	// information, use this when you create a problem for a pod
	defaultProblem := Resource{
		Owner: pod.Labels["reporting_team"],
		Name:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
		Type:  "pod",
	}

	// check if the pod has a problem from the enabled problems
	for _, problem := range enabledPodProblems {
		// Pass in Config
		resourceDetails, warning, occurring := problem.Detector(ctx, pod, o.cfg)
		if !occurring {
			continue
		}

		p := defaultProblem
		p.ProblemID = problem.ID
		p.ProblemDetails = resourceDetails
		p.Warning = warning
		problems = append(problems, p)
	}

	return problems, len(problems) > 0
}

// EDIT: New function
// getHPAsWithProblems creates a list of problem HPAs
func (o *Options) getHPAsWithProblems(ctx context.Context, hpa *v1.HorizontalPodAutoscaler) ([]Resource, bool) {
	problems := make([]Resource, 0)

	// defaultProblem is a problem that for the pod with prefilled
	// information, use this when you create a problem for a pod
	defaultProblem := Resource{
		Owner: hpa.Labels["reporting_team"],
		Name:  fmt.Sprintf("%s/%s", hpa.Namespace, hpa.Name),
		Type:  "HPA",
	}

	// check if the pod has a problem from the enabled problems
	for _, problem := range enabledHPAProblems {
		resourceDetails, warning, occurring := problem.Detector(ctx, hpa, o.cfg)
		if !occurring {
			continue
		}

		p := defaultProblem
		p.ProblemID = problem.ID
		p.ProblemDetails = resourceDetails
		p.Warning = warning
		problems = append(problems, p)
	}

	return problems, len(problems) > 0
}

// Run runs the devenv debug command
func (o *Options) Run(ctx context.Context) error { //nolint:funlen // Why: Best we can get currently
	//nolint:errcheck // Why: We handle errors
	k, err := kube.GetKubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to get kubernetes client (is the devenv running?)")
	}

	pods, err := k.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list pods")
	}

	// EDIT: Get HPAs
	HPAs, err := k.AutoscalingV1().HorizontalPodAutoscalers(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list hpas")
	}

	bold.Printf("Checking for problems ... ")
	resourceProblems := []Resource{}

	for i := range pods.Items {
		p := &pods.Items[i]
		if rs, is := o.getPodsWithProblems(ctx, p); is {
			resourceProblems = append(resourceProblems, rs...)
		}
	}

	// EDIT: Check HPAs
	for i := range HPAs.Items {
		h := &HPAs.Items[i]
		if rs, is := o.getHPAsWithProblems(ctx, h); is {
			resourceProblems = append(resourceProblems, rs...)
		}
	}

	bold.Println("done")
	if len(resourceProblems) == 0 {
		fmt.Println("Everything looks good 🎉")
		return nil
	}

	fmt.Println("")
	bold.Println("⛔️  Problems found (format: namespace/name <problem>):")

	report := ReportFromResources(resourceProblems)
	byProblem := report.ByProblem()
	bySeverity := report.BySeverity()

	for severity, problems := range bySeverity {
		for id, resources := range problems {
			p := report.GetProblemByID(id)
			if p == nil {
				continue
			}

			fmt.Println("")
			plural := ""
			if len(resources) > 1 {
				plural = "s"
			}

			// Get a color based on the severity
			var colorFn func(string, ...interface{}) string = color.HiRedString
			if severity == SeverityWarning {
				colorFn = color.HiYellowString
			}

			// Print the problem
			fmt.Printf("    %s %s\n",
				colorFn("%s: %s", id, p.ShortDescription),
				bold.Sprintf("[%d occurrence%s]",
					len(resources),
					plural,
				),
			)

			// Use a tabwriter so that the output is aligned
			tw := tabwriter.NewWriter(os.Stdout, 1, 0, 1, ' ', 0)
			for _, r := range resources {
				resourceMessage := bold.Sprint(r.Name)
				if r.ProblemDetails != "" {
					resourceMessage += ":\t" + r.ProblemDetails
				}
				if r.Owner != "" {
					resourceMessage += fmt.Sprintf(" (owned by %s)", r.Owner)
				}

				// Print the resource(s) that have the problem of this type
				fmt.Fprintln(tw, "    -", resourceMessage)
			}
			tw.Flush()
		}
	}

	fmt.Println()
	bold.Println("💡  More information/help:")
	tw := tabwriter.NewWriter(os.Stdout, 1, 0, 1, ' ', 0)
	for id := range byProblem {
		p := report.GetProblemByID(id)
		if p == nil {
			continue
		}

		helpURL := p.HelpURL
		if helpURL == "" {
			helpURL = "https://github.com/getoutreach/devenv/wiki/" + id
		}
		fmt.Fprintln(tw, "    -", bold.Sprint(id)+":\t", underline.Sprintf(helpURL))
	}
	tw.Flush()

	os.Exit(1)

	return nil
}
