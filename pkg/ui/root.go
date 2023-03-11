// Copyright 2023 Ewout Prangsma
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

package ui

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/binkynet/LocalWorker/pkg/ui/filepicker"
)

type Root struct {
	term    string
	width   int
	height  int
	loadAvg string

	selectFile struct {
		active bool
		picker filepicker.Model
	}
	showFile struct {
		active   bool
		viewPort viewport.Model
	}
}

var _ tea.Model = Root{}

// Init is the first function that will be called. It returns an optional
// initial command. To not perform an initial command return nil.
func (r Root) Init() tea.Cmd {
	return doReloadCPULoadAvg()
}

// Update is called when a message is received. Use it to inspect messages
// and, in response, update the model and/or send a command.
func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case loadAvgMsg:
		r.loadAvg = string(msg)
		return r, doReloadCPULoadAvg()
	case tea.WindowSizeMsg:
		r.height = msg.Height
		r.width = msg.Width
	case filepicker.FileSelectedMsg:
		r = r.openFile(string(msg))
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return r, tea.Quit
		case "k":
			r = r.openFile("/proc/kmsg")
		case "m":
			r = r.openFile("/proc/meminfo")
		case "v":
			r.showFile.active = false
			r.selectFile.picker = filepicker.New("/")
			r.selectFile.picker.Height = r.height - lipgloss.Height(r.headerView())
			r.selectFile.active = true
			cmds = append(cmds, r.selectFile.picker.Init())
		case "esc":
			r.showFile.active = false
		case "r":
			os.Exit(0)
		}
	}

	// Handle events in file picker
	if r.selectFile.active {
		var cmd tea.Cmd
		r.selectFile.picker, cmd = r.selectFile.picker.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Handle keyboard and mouse events in the viewport
	if r.showFile.active {
		var cmd tea.Cmd
		r.showFile.viewPort, cmd = r.showFile.viewPort.Update(msg)
		cmds = append(cmds, cmd)
	}

	return r, tea.Batch(cmds...)
}

// View renders the program's UI, which is just a string. The view is
// rendered after every Update.
func (r Root) View() string {
	s := r.headerView()
	if r.selectFile.active {
		return s + r.selectFile.picker.View()
	}
	if r.showFile.active {
		return s + r.showFile.viewPort.View()
	}
	s += `k - View /proc/kmsg
m - View /proc/meminfo
v - View file
r - Reboot
q - Disconnect
`
	return s
}

func (r Root) headerView() string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		"Welcome to BinkyNet Local worker!",
		r.loadAvg,
	) + "\n"
}

func (r Root) openFile(path string) Root {
	r.selectFile.active = false
	headerHeight := lipgloss.Height(r.headerView())
	verticalMarginHeight := 0
	useHighPerformanceRenderer := false

	content, _ := ioutil.ReadFile(path)
	r.showFile.viewPort = viewport.New(r.width, r.height-verticalMarginHeight)
	r.showFile.viewPort.YPosition = headerHeight
	r.showFile.viewPort.HighPerformanceRendering = useHighPerformanceRenderer
	r.showFile.viewPort.SetContent(string(content))
	r.showFile.active = true

	return r
}

type loadAvgMsg string

func doReloadCPULoadAvg() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		if content, err := ioutil.ReadFile("/proc/loadavg"); err != nil {
			return loadAvgMsg(err.Error())
		} else {
			return loadAvgMsg(string(content))
		}
	})
}
