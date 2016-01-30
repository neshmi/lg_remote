package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codegangsta/cli"
)

// Default port for LG TV API
const Port string = "8080"

// Default Base URI for LG TV
const BaseURI string = "/roap/api"

// TV record from JSON configuration file
type TV struct {
	Name           string `json:"name"`
	IP             string `json:"ip"`
	Key            string `json:"key"`
	Current3DState string
	Session        string
}

// TVConfig is a collection of TV records from the JSON configuration file
type TVConfig struct {
	TVs []TV
}

// GetAllTVs builds the TVConfig (and TVs) from the JSON file
func GetAllTVs() []TV {
	filename, _ := filepath.Abs("./tv_config.json")
	jsonFile, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	var tvConfig TVConfig

	err = json.Unmarshal(jsonFile, &tvConfig)
	if err != nil {
		panic(err)
	}
	return tvConfig.TVs
}

//BuildURI returns the complete URI string
func BuildURI(tv *TV, path string) string {
	uri := "http://" + tv.IP + ":" + Port + BaseURI + path
	return uri
}

//Check3D will check to see if a TV is currently in 3D mode
func (tv *TV) Check3D() bool {
	url := BuildURI(tv, "/data?target=is_3d")
	type Result struct {
		XMLName xml.Name `xml:"envelope"`
		Is3D    string   `xml:"data>is3D"`
	}
	v := Result{}

	resp, httperr := http.Get(url)
	if httperr != nil {
		panic(httperr)
	}

	body, readerr := ioutil.ReadAll(resp.Body)
	if readerr == nil {
		xml.Unmarshal(body, &v)
		switch v.Is3D {
		case "true":
			tv.Current3DState = "on"
		case "false":
			tv.Current3DState = "off"
		default:
			tv.Current3DState = "unknown"
		}
		return true
	}

	return false
}

// SendXML will post XML to the TV and return teh response
func (tv *TV) SendXML(data string, path string) (response *http.Response, err error) {
	// or you can use []byte(`...`) and convert to Buffer later on
	// build a new request, but not doing the POST yet
	url := BuildURI(tv, path)
	bodyReader := strings.NewReader(data)
	resp, err := http.Post(url, "atom+xml", bodyReader)

	return resp, err
}

// DisplayPairingKey causes the pairing key to be displayed on the passed TV object
func (tv *TV) DisplayPairingKey() bool {
	commandBody := `<!--?xml version=\"1.0\" encoding=\"utf-8\"?--><auth><type>AuthKeyReq</type></auth>`

	type Result struct {
		XMLName xml.Name `xml:"envelope"`
		Success string   `xml:"ROAPErrorDetail"`
	}

	v := Result{}

	resp, xmlerror := tv.SendXML(commandBody, "/auth")

	if xmlerror != nil {
		return false
	}

	body, readerr := ioutil.ReadAll(resp.Body)
	if readerr == nil {
		xml.Unmarshal(body, &v)
	}

	return resp.StatusCode == 200 && v.Success == "OK"
}

// SendCommand to TV, 400 activates the 3D mode, 20 is the okay button
func (tv *TV) SendCommand(command string) bool {
	if tv.Session == "" {
		if !tv.GetTVSession() {
			fmt.Printf("%s could not get session\n", tv.Name)
			return false
		}
	}

	commandBody := fmt.Sprintf(`<!--?xml version="1.0" encoding="utf-8"?--><command><name>HandleKeyInput</name><value>%s</value></command>`, command)

	type Result struct {
		XMLName xml.Name `xml:"envelope"`
		Success string   `xml:"ROAPErrorDetail"`
	}

	v := Result{}

	resp, xmlerror := tv.SendXML(commandBody, "/command")
	if xmlerror != nil {
		return false
	}
	body, readerr := ioutil.ReadAll(resp.Body)
	if readerr == nil {
		xml.Unmarshal(body, &v)
	}

	return v.Success == "OK"
}

//Enable3D enables 3D mode if TV not in 3D mode
func (tv *TV) Enable3D() bool {
	if tv.Current3DState == "on" {
		fmt.Printf("%s 3D Already Enabled\n", tv.Name)
		return true
	}
	enableResponse := tv.SendCommand("400")
	time.Sleep(1)
	okResponse := tv.SendCommand("20")
	if enableResponse && okResponse == true {
		tv.Current3DState = "on"
		fmt.Printf("%s 3D Enabled\n", tv.Name)
		return true
	}
	return false
}

//Disable3D disables 3D mode if currently in 3D
func (tv *TV) Disable3D() bool {
	if tv.Current3DState == "off" {
		fmt.Printf("%s 3D Already Disabled\n", tv.Name)
		return true
	}
	disableResponse := tv.SendCommand("400")
	if disableResponse == true {
		tv.Current3DState = "off"
		fmt.Printf("%s 3D Disabled\n", tv.Name)
		return true
	}
	return false
}

//GetTVSession authorizes a Session
func (tv *TV) GetTVSession() bool {
	// abort if pairing key isn't present
	if tv.Key == "" {
		fmt.Printf("%s No Pairing Key, set key first\n", tv.Name)
		return false
	}

	commandBody := fmt.Sprintf(`<!--?xml version="1.0" encoding="utf-8"?--><auth><type>AuthReq</type><value>%s</value></auth>`, tv.Key)

	type Result struct {
		XMLName   xml.Name `xml:"envelope"`
		Success   string   `xml:"ROAPErrorDetail"`
		SessionID string   `xml:"session"`
	}

	v := Result{}

	resp, senderror := tv.SendXML(commandBody, "/auth")
	if senderror != nil {
		fmt.Println("Error sending command")
		return false
	}

	body, readerr := ioutil.ReadAll(resp.Body)
	if readerr == nil && body != nil {
		xml.Unmarshal(body, &v)
		if v.Success == "OK" {
			tv.Session = v.SessionID
			return true
		}
		return false
	}
	return false
}

//FindTvByName will return a TV from the TVConfig collection
func FindTvByName(name string, tvs []TV) (tv *TV) {
	for _, tvElement := range tvs {
		if tvElement.Name == name {
			return &tvElement
		}
	}
	return &TV{}
}

func main() {
	app := cli.NewApp()
	app.Name = "LG Multi-screen Remote"
	app.Usage = "Control a cluster of LG Smart TVs"
	tvs := GetAllTVs()

	app.Commands = []cli.Command{
		{
			Name:    "enable",
			Aliases: []string{"e"},
			Usage:   "enable 3D on tv [tv name or all]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {
					for _, tv := range tvs {
						go tv.Enable3D()
					}
				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					tv.Enable3D()
				}
			},
		},
		{
			Name:    "disable",
			Aliases: []string{"d"},
			Usage:   "disable 3D on tv [tv name]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {
					for _, tv := range tvs {
						go tv.Disable3D()
					}
				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					tv.Disable3D()
				}
			},
		},
	}

	app.Run(os.Args)
}
