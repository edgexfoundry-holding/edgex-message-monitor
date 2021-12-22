// Copyright 2021 Alessandro De Blasis <alex@deblasis.net>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package data

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/deblasis/edgex-foundry-datamonitor/config"
	log "github.com/sirupsen/logrus"
)

func StringNotEmptyValidator(s string) error {
	if s == "" {
		return errors.New("Should not be empty")
	}
	return nil
}

func MinMaxValidator(min, max int, validationError error) func(s string) error {
	return func(s string) error {
		log.Debugf("validating %v", s)
		n, err := strconv.Atoi(s)
		if err != nil || n < min || n > max {
			return validationError
		}
		return nil
	}
}

func MustBeEmptyOrMatchLengthValidator(length int, validationError error) func(s string) error {
	return func(s string) error {
		l := len(s)
		if l != 0 && l != length {
			return validationError
		}
		return nil
	}
}

func MustGenerateValidRegexValidator(prefix string, s string) error {
	rx, err := compileTopicFilterRegex(fmt.Sprintf("%s%s", prefix, s))

	log.Debugf("REGEX %v", rx)
	return err
}

func compileTopicFilterRegex(filter string) (*regexp.Regexp, error) {
	//sanitization (order matters)
	f := strings.ReplaceAll(filter, "\\", "\\\\")
	f = strings.ReplaceAll(f, ".", "\\.")
	f = strings.ReplaceAll(f, "*", "\\*")
	f = strings.ReplaceAll(f, "#", ".*")
	f = strings.ReplaceAll(f, "^", "\\^")
	f = strings.ReplaceAll(f, "$", "\\$")
	f = strings.ReplaceAll(f, "{", "\\{")
	f = strings.ReplaceAll(f, "}", "\\}")
	f = strings.ReplaceAll(f, "(", "\\(")
	f = strings.ReplaceAll(f, ")", "\\)")
	f = strings.ReplaceAll(f, "+", "\\+")

	return regexp.Compile(fmt.Sprintf("(?i)^%s$", f)) //case insensitive
}

func MustCompileTopicFilterRegex(filter string) *regexp.Regexp {
	rx, _ := compileTopicFilterRegex(filter)
	return rx
}

var (
	ErrInvalidBufferSize   = fmt.Errorf("Must be a number between %d - %d", config.MinBufferSize, config.MaxBufferSize)
	ErrRedisPasswordLength = fmt.Errorf("If set, must be of length %d", config.RedisPasswordLength)
)
