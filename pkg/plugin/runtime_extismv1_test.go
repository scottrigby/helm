/*
Copyright The Helm Authors.
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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntimeConfigExtismV1Validate(t *testing.T) {

	rc := RuntimeConfigExtismV1{}
	err := rc.Validate()
	assert.NoError(t, err, "expected no error for empty RuntimeConfigExtismV1")
}

func TestRuntimeExtismV1CreatePlugin(t *testing.T) {
	r := RuntimeExtismV1{}

	metadata := &Metadata{
		APIVersion:    "v1",
		Name:          "testPlugin",
		Version:       "0.1.0",
		Type:          "test",
		Runtime:       "extismv1",
		RuntimeConfig: &RuntimeConfigExtismV1{},
	}
	p, err := r.CreatePlugin("test/", metadata)

	assert.NoError(t, err, "expected no error creating plugin")
	assert.NotNil(t, p, "expected plugin to be created")
}
