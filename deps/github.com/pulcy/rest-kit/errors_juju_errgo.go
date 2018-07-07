// Copyright (c) 2017 Epracom Advies.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !no_errgo

package restkit

import "github.com/juju/errgo"

func init() {
	WithStack = func(err error) error {
		err = errgo.Mask(err, errgo.Any)
		if e, _ := err.(*errgo.Err); e != nil {
			e.SetLocation(1)
		}
		return err
	}
	Cause = func(err error) error {
		return errgo.Cause(err)
	}
}
