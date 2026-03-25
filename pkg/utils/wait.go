/*
Copyright © 2021 Antoine Martin <antoine@openance.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// cSpell: words apimachinery
package utils

import (
	"context"
	"fmt"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kaweezle/iknite/pkg/cmd/options"
)

type WaitOptions struct {
	// Wait timeout in seconds
	Timeout time.Duration
	// Timeout for individual health checks
	CheckTimeout time.Duration
	// Interval between checks
	Interval time.Duration
	// Number of retries before giving up
	Retries int
	// Number of successful responses required to consider the check successful
	OkResponses int
	// Wait until ready (equivalent to infinite timeout)
	Wait bool
	// Watch the status continuously until interrupted (wait and infinite ok responses)
	Watch bool
	// If true, the condition will be evaluated immediately before starting the interval loop
	Immediate bool
}

func NewWaitOptions() *WaitOptions {
	return &WaitOptions{
		Timeout:      0,
		CheckTimeout: 10 * time.Second,
		Interval:     2 * time.Second,
		Retries:      3,
		OkResponses:  1,
		Watch:        false,
		Wait:         true,
		Immediate:    true,
	}
}

func InitializeWaitOptions(waitOptions *WaitOptions) {
	waitOptions.Timeout = 0
	waitOptions.CheckTimeout = 10 * time.Second
	waitOptions.Interval = 2 * time.Second
	waitOptions.Retries = 3
	waitOptions.OkResponses = 1
	waitOptions.Watch = false
	waitOptions.Wait = true
	waitOptions.Immediate = true
}

func AddWaitOptionsFlags(flags *flag.FlagSet, waitOptions *WaitOptions) {
	flags.DurationVarP(&waitOptions.Timeout, options.Timeout, "t", waitOptions.Timeout, "Wait timeout in seconds")
	flags.DurationVar(
		&waitOptions.CheckTimeout,
		options.CheckTimeout,
		waitOptions.CheckTimeout,
		"Timeout for individual health checks",
	)
	flags.DurationVar(&waitOptions.Interval, options.CheckInterval, waitOptions.Interval, "Interval between checks")
	flags.IntVar(&waitOptions.Retries, options.CheckRetries, waitOptions.Retries, "Number of retries before giving up")
	flags.IntVar(
		&waitOptions.OkResponses,
		options.CheckOkResponses,
		waitOptions.OkResponses,
		"Number of successful responses required to consider the check successful",
	)
	flags.BoolVarP(
		&waitOptions.Watch,
		options.Watch,
		"w",
		waitOptions.Watch,
		"Watch the status continuously until interrupted",
	)
	flags.BoolVarP(
		&waitOptions.Wait,
		options.Wait,
		"W",
		waitOptions.Wait,
		"Wait until ready (equivalent to infinite timeout)",
	)
	flags.BoolVar(
		&waitOptions.Immediate,
		options.CheckImmediate,
		waitOptions.Immediate,
		"If true, the condition will be evaluated immediately before starting the interval loop",
	)
}

func (waitOptions *WaitOptions) Validate() error {
	// if watch, timeout should be 0 and wait should be false
	if waitOptions.Watch {
		waitOptions.Timeout = 0  // override timeout to avoid conflicts
		waitOptions.Wait = false // override wait to avoid conflicts
	}
	// if wait, timeout should be 0 and watch should be false
	if waitOptions.Wait {
		waitOptions.Timeout = 0 // override timeout to avoid conflicts
	}
	return nil
}

func (waitOptions *WaitOptions) HasLoop() bool {
	return waitOptions.Watch || waitOptions.Wait || (waitOptions.Timeout > 0)
}

func (waitOptions *WaitOptions) String() string {
	return fmt.Sprintf(
		"WaitOptions{Timeout: %s, CheckTimeout: %s, Interval: %s, Retries: %d, OkResponses: %d, Watch: %t, Wait: %t}",
		waitOptions.Timeout,
		waitOptions.CheckTimeout,
		waitOptions.Interval,
		waitOptions.Retries,
		waitOptions.OkResponses,
		waitOptions.Watch,
		waitOptions.Wait,
	)
}

func (waitOptions *WaitOptions) Poll(ctx context.Context, condition wait.ConditionWithContextFunc) error {
	if err := waitOptions.Validate(); err != nil {
		return err
	}
	if !waitOptions.HasLoop() {
		_, err := condition(ctx)
		return err
	}
	actualCondition := condition
	actualContext := ctx
	if waitOptions.Retries > 0 {
		retries := waitOptions.Retries
		actualCondition = func(ctx context.Context) (bool, error) {
			if retries <= 0 {
				return false, context.DeadlineExceeded
			}
			done, err := condition(ctx)
			if err != nil {
				retries--
				err = nil
			}
			return done, err
		}
	}
	if waitOptions.Watch {
		// If watch is enabled, we want to ignore the condition's return value and only stop on error or context
		// cancellation
		watchCondition := actualCondition
		actualCondition = func(ctx context.Context) (bool, error) {
			_, err := watchCondition(ctx)
			if err != nil {
				return false, err
			}
			return false, nil // never stop based on condition, only on context cancellation
		}
	} else if waitOptions.Timeout > 0 {
		// If wait is not enabled and a timeout is set, we want to use a context with timeout
		var cancel context.CancelFunc
		actualContext, cancel = context.WithTimeout(ctx, waitOptions.Timeout)
		defer cancel()
	}
	if !waitOptions.Watch && waitOptions.OkResponses > 1 { //nolint:nestif // kept nested for readability
		// If we need multiple successful responses, we wrap the condition to check for the required number of successes
		successes := 0
		okCondition := actualCondition
		actualCondition = func(ctx context.Context) (bool, error) {
			ok, err := okCondition(ctx)
			if err != nil {
				successes = 0 // reset successes on error
				return false, err
			}
			if ok {
				successes++
				if successes >= waitOptions.OkResponses {
					return true, nil
				}
			} else {
				successes = 0 // reset successes if condition is not met
			}
			return false, nil
		}
	}
	err := wait.PollUntilContextCancel(actualContext, waitOptions.Interval, waitOptions.Immediate, actualCondition)
	if err != nil {
		return fmt.Errorf("condition not met within the specified options: %w", err)
	}
	return nil
}
