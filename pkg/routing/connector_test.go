package routing


import (
	"goose/pkg/routing"

)

// test wire connect close
func TestReConnect(t *testing.T) {

	connector, err := routing.NewConnector()
	if err != nil {
		f.Log(err)
		t.Fail()
	}
	endpoint := 
	connector.Connect(endpoint)

}