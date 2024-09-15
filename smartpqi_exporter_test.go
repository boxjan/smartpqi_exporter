// Copyright 2015 The Prometheus Authors
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
package main

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"os"
	"testing"
)

func TestParserData(T *testing.T) {
	e := Exporter{
		logger:                  log.NewLogfmtLogger(os.Stderr),
		collectSMART:            true,
		skipCollectSMARTForJDOB: false,
	}

	for i := 1; i <= 3; i++ {
		ctrl, err := os.ReadFile(fmt.Sprintf("tests/arcconf-%d-getconfigjson", i))
		if err != nil {
			T.Errorf("Error reading file: %s", err)
			return
		}
		smart, err := os.ReadFile(fmt.Sprintf("tests/arcconf-%d-getsmartstats", i))
		if err != nil {
			T.Errorf("Error reading file: %s", err)
			return
		}
		metrics := make(chan prometheus.Metric, 1024)
		go func() {
			e.parserData(ctrl, smart, metrics)
			close(metrics)
		}()
		for {
			if metric, ok := <-metrics; !ok {
				break
			} else {
				m := &io_prometheus_client.Metric{}
				_ = metric.Write(m)
				fmt.Println(metric.Desc(), m.String())
			}
		}

	}

}
