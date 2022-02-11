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
	"testing"

	"github.com/stretchr/testify/require"
)

var expectedTestPolicy = `##########
## First, we generate additional policy for the API that will allow us to manage access for the specific service.
##########

p, system:crdb:test,                /api/v1/credentials/crdb/test,     GET
p, admin:varys:services:crdb:test,  /api/v1/services/crdb/test/grants, (GET)|(PUT)|(DELETE)
p, update:varys:services:crdb:test, /api/v1/services/crdb/test,        PUT
p, delete:varys:services:crdb:test, /api/v1/services/crdb/test,        DELETE

##########
## Next, assign the newly generated policies to existing, higher level groups for the service kind.
##########

g, system:crdb,                system:crdb:test
g, admin:varys:services:crdb,  admin:varys:services:crdb:test
g, admin:varys:services:crdb,  admin:varys:services:crdb:test
g, update:varys:services:crdb, update:varys:services:crdb:test
g, delete:varys:services:crdb, delete:varys:services:crdb:test
g, read:crdb,                  read:crdb:test
g, write:crdb,                 write:crdb:test
g, update:crdb,                update:crdb:test
g, delete:crdb,                delete:crdb:test
g, admin:crdb,                 admin:crdb:test

##########
## Finally, we assign the creator of the service permissions to update, delete, and administer the service they just
## created within the system.
##########

g, /_user/basic/OHDJ3W5OWTU63XHPY466XHV2OHDXJY5XTPN5N7D3JU36D3N3PHPBU36ND5VV3WWXX34X7LJU5GWXW, admin:varys:services:crdb:test
g, /_user/basic/OHDJ3W5OWTU63XHPY466XHV2OHDXJY5XTPN5N7D3JU36D3N3PHPBU36ND5VV3WWXX34X7LJU5GWXW, update:varys:services:crdb:test
g, /_user/basic/OHDJ3W5OWTU63XHPY466XHV2OHDXJY5XTPN5N7D3JU36D3N3PHPBU36ND5VV3WWXX34X7LJU5GWXW, delete:varys:services:crdb:test
`

func TestRenderServicePolicy(t *testing.T) {
	rendered, err := renderServicePolicy(policyTemplate{
		Service: Service{
			Kind: "crdb",
			Name: "test",
		},
		Creator: User{
			Kind: "basic",
			ID:   "OHDJ3W5OWTU63XHPY466XHV2OHDXJY5XTPN5N7D3JU36D3N3PHPBU36ND5VV3WWXX34X7LJU5GWXW",
			Name: "badadmin",
		},
	})

	require.NoError(t, err)
	require.Equal(t, expectedTestPolicy, rendered)
}
