// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.
//
// Package main is the entrypoint for the CLI.
package main

import (
	"github.com/alecthomas/kong"
	"github.com/runatlantis/atlantis/cmd"
)

const atlantisVersion = "0.17.3"

func main() {
	ctx := kong.Parse(
		&cmd.CLI,
		cmd.FlagsVars,
		kong.DefaultEnvars("ATLANTIS"),
		kong.Bind(cmd.Context{
			Version: atlantisVersion,
		}),
	)
	err := ctx.Run(&cmd.Context{
		Version: atlantisVersion,
	})
	ctx.FatalIfErrorf(err)
}
