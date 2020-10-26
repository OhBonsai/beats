// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package urlparse

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/processors"
	"github.com/elastic/beats/v7/libbeat/processors/checks"
	jsprocessor "github.com/elastic/beats/v7/libbeat/processors/script/javascript/module/processor"
)

type urlParse struct {
	config urlParseConfig
	log    *logp.Logger
}

type urlParseConfig struct {
	Fields        []fromTo `config:"fields" validate:"required"`
	IgnoreMissing bool     `config:"ignore_missing"`
	FailOnError   bool     `config:"fail_on_error"`
}

type fromTo struct {
	From string `config:"from" validate:"required"`
	To   string `config:"to"`
}

func init() {
	processors.RegisterPlugin("urlparse",
		checks.ConfigChecked(New,
			checks.RequireFields("fields"),
			checks.AllowedFields("fields", "ignore_missing", "fail_on_error")))
	jsprocessor.RegisterPlugin("URLParse", New)
}

func New(c *common.Config) (processors.Processor, error) {
	config := urlParseConfig{
		IgnoreMissing: false,
		FailOnError:   true,
	}

	if err := c.Unpack(&config); err != nil {
		return nil, fmt.Errorf("failed to unpack the configuration of urlparse processor: %s", err)
	}

	return &urlParse{
		config: config,
		log:    logp.NewLogger("urlparse"),
	}, nil

}

func (p *urlParse) Run(event *beat.Event) (*beat.Event, error) {
	var backup common.MapStr
	if p.config.FailOnError {
		backup = event.Fields.Clone()
	}

	for _, field := range p.config.Fields {
		err := p.parseField(field.From, field.To, event)
		if err != nil {
			errMsg := fmt.Errorf("failed to parse fields in urlparse processor: %v", err)
			p.log.Debug(errMsg.Error())
			if p.config.FailOnError {
				event.Fields = backup
				event.PutValue("error.message", errMsg.Error())
				return event, err
			}
		}
	}

	return event, nil
}

func (p *urlParse) parseField(from string, to string, event *beat.Event) error {
	value, err := event.GetValue(from)
	if err != nil {
		if p.config.IgnoreMissing && errors.Cause(err) == common.ErrKeyNotFound {
			return nil
		}
		return fmt.Errorf("could not fetch value for key: %s, Error: %v", from, err)
	}

	parsedString, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid type for `from`, expecting a string received %T", value)
	}
	parseData, err := url.Parse(parsedString)

	if err != nil {
		return fmt.Errorf("error trying to URL-parse %s: %v", parsedString, err)
	}

	target := to
	if to == "" {
		target = from
	}

	if _, err := event.PutValue(target, common.MapStr{
		"scheme":    parseData.Scheme,
		"opaque":    parseData.Opaque,
		"hostname":  parseData.Hostname(),
		"port":      parseData.Port(),
		"path":      parseData.Path,
		"raw_path":  parseData.RawPath,
		"raw_query": parseData.RawQuery,
		"fragment":  parseData.Fragment,
	}); err != nil {
		return fmt.Errorf("could not put value: %s: %v, %v", parseData, target, err)
	}

	return nil
}

func (p *urlParse) String() string {
	return "urlparse=" + fmt.Sprintf("%+v", p.config.Fields)
}
