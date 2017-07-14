//    Copyright 2017 Ewout Prangsma
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package bridge

// API of the bridge, the hardware used to connect a Raspberry PI GPIO
// port to a buffered I2C network that local slaves are connected to.
type API interface {
	// Turn Green status led on/off
	SetGreenLED(on bool) error
	// Turn Red status led on/off
	SetRedLED(on bool) error
}
