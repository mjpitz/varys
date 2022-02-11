// Copyright (C) 2022  Mya Pitzeruse
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

package engine

import (
	"bytes"
	_ "embed"
	"sync"
	"text/template"
)

var (
	// Model defines the policy model definition used by the engine.
	//go:embed casbin_model.conf
	Model string

	// DefaultPolicy defines the default policy used by the system.
	//go:embed casbin_default_policy.csv
	DefaultPolicy string

	//go:embed casbin_service_policy.tmpl.csv
	servicePolicyTemplate string

	t     *template.Template
	tInit sync.Once
)

func renderServicePolicy(policy policyTemplate) (string, error) {
	tInit.Do(func() {
		t = template.Must(template.New("casbin_service_policy").Parse(servicePolicyTemplate))
	})

	render := bytes.NewBuffer(nil)
	err := t.Execute(render, policy)
	if err != nil {
		return "", err
	}

	return render.String(), nil
}

type policyTemplate struct {
	Service Service
	Creator User
}
