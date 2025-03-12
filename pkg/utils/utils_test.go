/*
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

package utils

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation"
	"testing"
)

func TestSanitizeLabelValue(t *testing.T) {
	tests := []string{
		"valid-value",
		"invalid_value!",
		"",
		"123",
		".invalid-start",
		"toolongvaluexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}

	for _, test := range tests {
		converted := SanitizeLabelValue(test)
		isValid := len(validation.IsValidLabelValue(converted)) == 0
		fmt.Printf("Original: %-70s Converted: %-70s Valid: %t\n", test, converted, isValid)
	}
}
