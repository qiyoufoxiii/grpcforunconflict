/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpcforunconflict

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/qiyouForSql/grpcforunconflict/balancer"
	"github.com/qiyouForSql/grpcforunconflict/serviceconfig"
)

type parseTestCase struct {
	scjs    string
	wantSC  *ServiceConfig
	wantErr bool
}

func runParseTests(t *testing.T, testCases []parseTestCase) {
	t.Helper()
	for _, c := range testCases {
		scpr := parseServiceConfig(c.scjs)
		var sc *ServiceConfig
		sc, _ = scpr.Config.(*ServiceConfig)
		if !c.wantErr {
			c.wantSC.rawJSONString = c.scjs
		}
		if c.wantErr != (scpr.Err != nil) || !reflect.DeepEqual(sc, c.wantSC) {
			t.Fatalf("parseServiceConfig(%s) = %+v, %v, want %+v, %v", c.scjs, sc, scpr.Err, c.wantSC, c.wantErr)
		}
	}
}

type pbbData struct {
	serviceconfig.LoadBalancingConfig
	Foo string
	Bar int
}

type parseBalancerBuilder struct{}

func (parseBalancerBuilder) Name() string {
	return "pbb"
}

func (parseBalancerBuilder) ParseConfig(c json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	d := pbbData{}
	if err := json.Unmarshal(c, &d); err != nil {
		return nil, err
	}
	return d, nil
}

func (parseBalancerBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	panic("unimplemented")
}

func init() {
	balancer.Register(parseBalancerBuilder{})
}

func (s) TestParseLBConfig(t *testing.T) {
	testcases := []parseTestCase{
		{
			`{
    "loadBalancingConfig": [{"pbb": { "foo": "hi" } }]
}`,
			&ServiceConfig{
				Methods:  make(map[string]MethodConfig),
				lbConfig: &lbConfig{name: "pbb", cfg: pbbData{Foo: "hi"}},
			},
			false,
		},
	}
	runParseTests(t, testcases)
}

func (s) TestParseNoLBConfigSupported(t *testing.T) {
	// We have a loadBalancingConfig field but will not encounter a supported
	// policy.  The config will be considered invalid in this case.
	testcases := []parseTestCase{
		{
			scjs: `{
    "loadBalancingConfig": [{"not_a_balancer1": {} }, {"not_a_balancer2": {}}]
}`,
			wantErr: true,
		}, {
			scjs:    `{"loadBalancingConfig": []}`,
			wantErr: true,
		},
	}
	runParseTests(t, testcases)
}

func (s) TestParseLoadBalancer(t *testing.T) {
	testcases := []parseTestCase{
		{
			`{
    "loadBalancingPolicy": "round_robin",
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "waitForReady": true
        }
    ]
}`,
			&ServiceConfig{
				LB: newString("round_robin"),
				Methods: map[string]MethodConfig{
					"/foo/Bar": {
						WaitForReady: newBool(true),
					},
				},
			},
			false,
		},
		{
			`{
    "loadBalancingPolicy": 1,
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "waitForReady": false
        }
    ]
}`,
			nil,
			true,
		},
	}
	runParseTests(t, testcases)
}

func (s) TestParseWaitForReady(t *testing.T) {
	testcases := []parseTestCase{
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "waitForReady": true
        }
    ]
}`,
			&ServiceConfig{
				Methods: map[string]MethodConfig{
					"/foo/Bar": {
						WaitForReady: newBool(true),
					},
				},
			},
			false,
		},
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "waitForReady": false
        }
    ]
}`,
			&ServiceConfig{
				Methods: map[string]MethodConfig{
					"/foo/Bar": {
						WaitForReady: newBool(false),
					},
				},
			},
			false,
		},
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "waitForReady": fall
        },
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "waitForReady": true
        }
    ]
}`,
			nil,
			true,
		},
	}

	runParseTests(t, testcases)
}

func (s) TestParseTimeOut(t *testing.T) {
	testcases := []parseTestCase{
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "timeout": "1s"
        }
    ]
}`,
			&ServiceConfig{
				Methods: map[string]MethodConfig{
					"/foo/Bar": {
						Timeout: newDuration(time.Second),
					},
				},
			},
			false,
		},
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "timeout": "3c"
        }
    ]
}`,
			nil,
			true,
		},
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "timeout": "3c"
        },
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "timeout": "1s"
        }
    ]
}`,
			nil,
			true,
		},
	}

	runParseTests(t, testcases)
}

func (s) TestParseMsgSize(t *testing.T) {
	testcases := []parseTestCase{
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "maxRequestMessageBytes": 1024,
            "maxResponseMessageBytes": 2048
        }
    ]
}`,
			&ServiceConfig{
				Methods: map[string]MethodConfig{
					"/foo/Bar": {
						MaxReqSize:  newInt(1024),
						MaxRespSize: newInt(2048),
					},
				},
			},
			false,
		},
		{
			`{
    "methodConfig": [
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "maxRequestMessageBytes": "1024",
            "maxResponseMessageBytes": "2048"
        },
        {
            "name": [
                {
                    "service": "foo",
                    "method": "Bar"
                }
            ],
            "maxRequestMessageBytes": 1024,
            "maxResponseMessageBytes": 2048
        }
    ]
}`,
			nil,
			true,
		},
	}

	runParseTests(t, testcases)
}
func (s) TestParseDefaultMethodConfig(t *testing.T) {
	dc := &ServiceConfig{
		Methods: map[string]MethodConfig{
			"": {WaitForReady: newBool(true)},
		},
	}

	runParseTests(t, []parseTestCase{
		{
			`{
  "methodConfig": [{
    "name": [{}],
    "waitForReady": true
  }]
}`,
			dc,
			false,
		},
		{
			`{
  "methodConfig": [{
    "name": [{"service": null}],
    "waitForReady": true
  }]
}`,
			dc,
			false,
		},
		{
			`{
  "methodConfig": [{
    "name": [{"service": ""}],
    "waitForReady": true
  }]
}`,
			dc,
			false,
		},
		{
			`{
  "methodConfig": [{
    "name": [{"method": "Bar"}],
    "waitForReady": true
  }]
}`,
			nil,
			true,
		},
		{
			`{
  "methodConfig": [{
    "name": [{"service": "", "method": "Bar"}],
    "waitForReady": true
  }]
}`,
			nil,
			true,
		},
	})
}

func (s) TestParseMethodConfigDuplicatedName(t *testing.T) {
	runParseTests(t, []parseTestCase{
		{
			`{
  "methodConfig": [{
    "name": [
      {"service": "foo"},
      {"service": "foo"}
    ],
    "waitForReady": true
  }]
}`, nil, true,
		},
	})
}

func newBool(b bool) *bool {
	return &b
}

func newDuration(b time.Duration) *time.Duration {
	return &b
}

func newString(b string) *string {
	return &b
}
