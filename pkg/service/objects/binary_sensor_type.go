// Copyright 2022 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package objects

import (
	"context"

	"github.com/rs/zerolog"
)

type binarySensorType string

const binarySensorTypeInstance binarySensorType = "binarySensorType"

func (t binarySensorType) String() string {
	return string(t)
}

func (binarySensorType) Run(ctx context.Context, log zerolog.Logger, requests RequestService, statuses StatusService, service Service, moduleID string) error {
	<-ctx.Done()
	return nil
}
