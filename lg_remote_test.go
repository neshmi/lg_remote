package main

import (
	"testing"

	"github.com/jarcoal/httpmock"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLgRemote(t *testing.T) {
	tv1 := &TV{Name: "TV-1", IP: "192.168.1.100", Key: "xyz123", Current3DState: "off"}
	tv2 := &TV{Name: "TV-2", IP: "192.168.1.101", Key: "123xyz", Current3DState: "off"}
	tv3 := &TV{Name: "TV-2", IP: "192.168.1.102", Key: "123xyz", Current3DState: "off"}

	Convey("Given a TV Configuration file", t, func() {
		tvsFromJSON := GetAllTVs()
		Convey("It should return an array of TVs and Codes", func() {
			// Loading from tv_config.json, 2 TVs in the config file
			So(tvsFromJSON, ShouldHaveLength, 2)
			// Test first part of the slice
			So(tvsFromJSON[0].Name, ShouldEqual, "TV-1")
			So(tvsFromJSON[0].IP, ShouldEqual, "192.168.1.100")
			So(tvsFromJSON[0].Key, ShouldEqual, "xyz123")
			// Test second part of the slice
			So(tvsFromJSON[1].Name, ShouldEqual, "TV-2")
			So(tvsFromJSON[1].IP, ShouldEqual, "192.168.1.101")
			So(tvsFromJSON[1].Key, ShouldEqual, "123xyz")
		})
	})

	Convey("Given a TV and URI path", t, func() {
		// tv1 := &TV{Name: "TV-1", IP: "192.168.1.100", Key: "xyz123"}

		Convey("It should return a complete URI path", func() {
			So(BuildURI(tv1, "/auth"), ShouldEqual, "http://192.168.1.100:8080/udap/api/auth")
		})
	})

	Convey("Given a TV without a pairing key", t, func() {
		Convey("It should display pairing key on the TV", func() {
			response := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>OK</ROAPErrorDetail><session>1051689385</session></envelope>
			`

			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			httpmock.RegisterResponder("POST", "http://192.168.1.100:8080/udap/api/auth", httpmock.NewStringResponder(200, response))

			httpmock.RegisterResponder("POST", "http://192.168.1.101:8080/udap/api/auth", httpmock.NewStringResponder(500, response))

			So(tv1.DisplayPairingKey(), ShouldEqual, true)
			So(tv2.DisplayPairingKey(), ShouldEqual, false)
		})

	})

	Convey("Given a TV", t, func() {
		Convey("It should query its 3D state", func() {
			falseMode := `
			<?xml version="1.0" encoding="utf-8"?>
			<envelope>
				<dataList name="is3D">
	        <data>
	            <is3D>false</is3D>
	        </data>
				</dataList>
			</envelope>
			`

			trueMode := `
			<?xml version="1.0" encoding="utf-8"?>
			<envelope>
				<dataList name="is3D">
	        <data>
	            <is3D>true</is3D>
	        </data>
				</dataList>
			</envelope>
			`

			blankMode := ``

			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			httpmock.RegisterResponder("GET", "http://192.168.1.100:8080/udap/api/data?target=is_3d", httpmock.NewStringResponder(200, falseMode))

			httpmock.RegisterResponder("GET", "http://192.168.1.101:8080/udap/api/data?target=is_3d", httpmock.NewStringResponder(200, trueMode))

			httpmock.RegisterResponder("GET", "http://192.168.1.102:8080/udap/api/data?target=is_3d", httpmock.NewStringResponder(200, blankMode))
			// Set Mock Server to return false for TV1
			// So(tv1.Is3D(), ShouldEqual, "false")
			So(tv1.Current3DState, ShouldEqual, "off")
			So(tv1.Check3D(), ShouldEqual, true)
			So(tv1.Current3DState, ShouldEqual, "off")

			// Set Mock Server to return true for TV2, should switch the state of the TV record
			So(tv2.Current3DState, ShouldEqual, "off")
			So(tv2.Check3D(), ShouldEqual, true)
			So(tv2.Current3DState, ShouldEqual, "on")

			// Set Mock Server to return true for TV3, should switch to uknown since response doesn't register correctly
			So(tv3.Current3DState, ShouldEqual, "off")
			So(tv3.Check3D(), ShouldEqual, true)
			So(tv3.Current3DState, ShouldEqual, "unknown")
		})

		Convey("It should enable the 3D", func() {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			authorizedSuccess := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>OK</ROAPErrorDetail><session>1051689385</session></envelope>
			`
			authorizedFail := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>FAIL</ROAPErrorDetail><session>1051689385</session></envelope>
			`
			unauthorized := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>FAIL</ROAPErrorDetail><session>1051689385</session></envelope>
			`

			httpmock.RegisterResponder("POST", "http://192.168.1.100:8080/udap/api/command", httpmock.NewStringResponder(200, authorizedSuccess))

			httpmock.RegisterResponder("POST", "http://192.168.1.101:8080/udap/api/command", httpmock.NewStringResponder(200, authorizedFail))

			httpmock.RegisterResponder("POST", "http://192.168.1.102:8080/udap/api/command", httpmock.NewStringResponder(200, unauthorized))

			tv1.Current3DState = "off"
			tv2.Current3DState = "off"
			tv3.Current3DState = "off"

			So(tv1.Enable3D(), ShouldEqual, true)
			So(tv1.Current3DState, ShouldEqual, "on")
			So(tv2.Enable3D(), ShouldEqual, false)
			So(tv2.Current3DState, ShouldEqual, "off")
			So(tv3.Enable3D(), ShouldEqual, false)
			So(tv3.Current3DState, ShouldEqual, "off")
		})

		Convey("It should disable the 3D", func() {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			authorizedSuccess := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>OK</ROAPErrorDetail><session>1051689385</session></envelope>
			`
			authorizedFail := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>FAIL</ROAPErrorDetail><session>1051689385</session></envelope>
			`
			unauthorized := `
			<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>FAIL</ROAPErrorDetail><session>1051689385</session></envelope>
			`

			httpmock.RegisterResponder("POST", "http://192.168.1.100:8080/udap/api/command", httpmock.NewStringResponder(200, authorizedSuccess))

			httpmock.RegisterResponder("POST", "http://192.168.1.101:8080/udap/api/command", httpmock.NewStringResponder(200, authorizedFail))

			httpmock.RegisterResponder("POST", "http://192.168.1.102:8080/udap/api/command", httpmock.NewStringResponder(200, unauthorized))

			tv1.Current3DState = "on"
			tv2.Current3DState = "on"
			tv3.Current3DState = "on"

			So(tv1.Disable3D(), ShouldEqual, true)
			So(tv1.Current3DState, ShouldEqual, "off")
			So(tv2.Disable3D(), ShouldEqual, false)
			So(tv2.Current3DState, ShouldEqual, "on")
			So(tv3.Disable3D(), ShouldEqual, false)
			So(tv3.Current3DState, ShouldEqual, "on")
		})

		Convey("It should get the session if it has a pairing key", func() {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			sessionSuccess := `
				<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>200</ROAPError><ROAPErrorDetail>OK</ROAPErrorDetail><session>1051689385</session></envelope>
			`

			sessionFail := `
				<?xml version="1.0" encoding="utf-8"?><envelope><ROAPError>404</ROAPError><ROAPErrorDetail>NotAuth</ROAPErrorDetail><session></session></envelope>
			`

			httpmock.RegisterResponder("POST", "http://192.168.1.100:8080/udap/api/auth", httpmock.NewStringResponder(200, sessionSuccess))

			httpmock.RegisterResponder("POST", "http://192.168.1.101:8080/udap/api/auth", httpmock.NewStringResponder(200, sessionFail))

			So(tv1.Session, ShouldEqual, "")
			So(tv1.GetTVSession(), ShouldEqual, true)
			So(tv1.Session, ShouldEqual, "1051689385")

			So(tv2.Session, ShouldEqual, "")
			So(tv2.GetTVSession(), ShouldEqual, false)
			So(tv2.Session, ShouldEqual, "")
		})

	})

}
