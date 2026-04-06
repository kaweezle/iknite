/*
Copyright © 2025 Antoine Martin <antoine@openance.com>

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
// cSpell: words sirupsen logrus
package util

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type LogLevelValue log.Level

// Ensure logLevelValue implements the pflag.Value interface.
var _ pflag.Value = (*LogLevelValue)(nil)

func NewLogLevelValue(p *log.Level) *LogLevelValue {
	return (*LogLevelValue)(p)
}

func (c *LogLevelValue) Set(s string) error {
	level, err := log.ParseLevel(s)
	if err != nil {
		return fmt.Errorf("while parsing log level: %w", err)
	}
	*c = LogLevelValue(level)
	return nil
}

func (s *LogLevelValue) Type() string {
	return "logLevel"
}

func (s *LogLevelValue) String() string { return log.Level(*s).String() }
