// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package builder

import (
	"os"
	"path"

	"github.com/goodrain/rainbond/util/constants"
)

func init() {
	if os.Getenv("BUILD_IMAGE_REPOSTORY_DOMAIN") != "" {
		REGISTRYDOMAIN = os.Getenv("BUILD_IMAGE_REPOSTORY_DOMAIN")
	}
	if os.Getenv("BUILD_IMAGE_REPOSTORY_USER") != "" {
		REGISTRYUSER = os.Getenv("BUILD_IMAGE_REPOSTORY_USER")
	}
	if os.Getenv("BUILD_IMAGE_REPOSTORY_PASS") != "" {
		REGISTRYPASS = os.Getenv("BUILD_IMAGE_REPOSTORY_PASS")
	}
	RUNNERIMAGENAME = "/runner"
	if os.Getenv("RUNNER_IMAGE_NAME") != "" {
		RUNNERIMAGENAME = os.Getenv("RUNNER_IMAGE_NAME")
	}
	RUNNERIMAGENAME = path.Join(REGISTRYDOMAIN, RUNNERIMAGENAME)
	BUILDERIMAGENAME = "builder"
	if os.Getenv("BUILDER_IMAGE_NAME") != "" {
		BUILDERIMAGENAME = os.Getenv("BUILDER_IMAGE_NAME")
	}

	BUILDERIMAGENAME = path.Join(REGISTRYDOMAIN, BUILDERIMAGENAME)
}

//REGISTRYDOMAIN REGISTRY_DOMAIN
var REGISTRYDOMAIN = constants.DefImageRepository

//REGISTRYUSER REGISTRY USER NAME
var REGISTRYUSER = ""

//REGISTRYPASS REGISTRY PASSWORD
var REGISTRYPASS = ""

//RUNNERIMAGENAME runner image name
var RUNNERIMAGENAME string

//BUILDERIMAGENAME builder image name
var BUILDERIMAGENAME string
