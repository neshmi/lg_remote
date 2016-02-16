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
	configPath := os.Getenv("LG_REMOTE_PATH")
	configFile := os.Getenv("LG_REMOTE_CONFIG_FILE")
	var filename string
	var fileerr error
	if configFile != "" && configPath != "" {
		filename = filepath.Join(configPath, configFile)
	} else {
		filename, fileerr = filepath.Abs("./tv_config.json")
	}
	if fileerr != nil {
		fmt.Println("Could not find TV Config file")
	}

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
	var okResponse bool

	enableResponse := tv.SendCommand("400")
	//only send the second command if the first has sent successfuly
	if enableResponse == true {
		time.Sleep(1)
		lrResponse := tv.SendCommand("401")
		if lrResponse == true {
			okResponse = tv.SendCommand("412")
		}
	}

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
		return true
	}
	disableResponse := tv.SendCommand("400")
	if disableResponse == true {
		tv.Current3DState = "off"
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
		// fmt.Println("Error sending command")
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
	app.Version = "0.0.1"
	tvs := GetAllTVs()

	app.Commands = []cli.Command{
		{
			Name:    "enable-3D",
			Aliases: []string{"e"},
			Usage:   "enable [tv name or all]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {

					done := make(chan bool)

					for _, tv := range tvs {
						tv := tv
						go func() {
							fmt.Printf("Enabling: %s\n", tv.Name)
							if tv.Enable3D() {
								fmt.Printf("%s: Enabled 3D\n", tv.Name)
							} else {
								fmt.Printf("%s: Failed\n", tv.Name)
							}
							done <- true
						}()
					}

					for _ = range tvs {
						<-done
					}

				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					if tv.Name == c.Args().First() {
						fmt.Printf("Enabling: %s\n", tv.Name)
						if tv.Enable3D() {
							fmt.Printf("%s: Enabled 3D\n", tv.Name)
						} else {
							fmt.Printf("%s: Failed\n", tv.Name)
						}
					} else {
						fmt.Printf("Couldn't find tv %s\n", c.Args().First())
					}
				}
			},
		},
		{
			Name:    "disable-3D",
			Aliases: []string{"d"},
			Usage:   "disable [tv name or all]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {
					done := make(chan bool)

					for _, tv := range tvs {
						tv := tv
						go func() {
							fmt.Printf("Disabling: %s\n", tv.Name)
							if tv.Disable3D() {
								fmt.Printf("%s: Disabled 3D\n", tv.Name)
							} else {
								fmt.Printf("%s: Failed\n", tv.Name)
							}
							done <- true
						}()
					}

					for _ = range tvs {
						<-done
					}
				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					if tv.Name == c.Args().First() {
						fmt.Printf("Disabling: %s\n", tv.Name)
						if tv.Disable3D() {
							fmt.Printf("%s: Disabled 3D\n", tv.Name)
						} else {
							fmt.Printf("%s: Failed\n", tv.Name)
						}
					} else {
						fmt.Printf("Couldn't find tv %s\n", c.Args().First())
					}
				}
			},
		},
		{
			Name:    "query-3D-state",
			Aliases: []string{"q"},
			Usage:   "query [tv name or all]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {
					done := make(chan bool)

					for _, tv := range tvs {
						tv := tv
						go func() {
							fmt.Printf("Checking: %s\n", tv.Name)
							if tv.Check3D() {
								fmt.Printf("%s 3D State: %s\n", tv.Name, tv.Current3DState)
							} else {
								fmt.Printf("%s: Failed\n", tv.Name)
							}
							done <- true
						}()
					}

					for _ = range tvs {
						<-done
					}
				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					if tv.Name == c.Args().First() {
						fmt.Printf("Checking: %s\n", tv.Name)
						if tv.Check3D() {
							fmt.Printf("%s 3D State: %s\n", tv.Name, tv.Current3DState)
						} else {
							fmt.Printf("%s: Failed\n", tv.Name)
						}
					} else {
						fmt.Printf("Couldn't find tv %s\n", c.Args().First())
					}
				}
			},
		},
		{
			Name:    "display-pairing-key",
			Aliases: []string{"r"},
			Usage:   "pair [tv name or all]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {
					done := make(chan bool)

					for _, tv := range tvs {
						tv := tv
						go func() {
							fmt.Printf("Display key for: %s\n", tv.Name)
							if tv.DisplayPairingKey() {
								fmt.Printf("Displaying...\n")
							} else {
								fmt.Printf("%s: Failed\n", tv.Name)
							}
							done <- true
						}()
					}

					for _ = range tvs {
						<-done
					}
				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					if tv.Name == c.Args().First() {
						fmt.Printf("Display key for: %s\n", tv.Name)
						if tv.DisplayPairingKey() {
							fmt.Printf("Displaying...\n")
						} else {
							fmt.Printf("%s: Failed\n", tv.Name)
						}
					} else {
						fmt.Printf("Couldn't find tv %s\n", c.Args().First())
					}
				}
			},
		},
		{
			Name:    "power-off",
			Aliases: []string{"p"},
			Usage:   "pair [tv name or all]",
			Action: func(c *cli.Context) {
				if c.Args().First() == "all" {
					done := make(chan bool)

					for _, tv := range tvs {
						tv := tv
						go func() {
							fmt.Printf("Powering off: %s\n", tv.Name)
							if tv.SendCommand("1") {
								fmt.Printf("Powered off %s", tv.Name)
							} else {
								fmt.Printf("%s: Failed\n", tv.Name)
							}
							done <- true
						}()
					}

					for _ = range tvs {
						<-done
					}
				} else {
					tv := FindTvByName(c.Args().First(), tvs)
					if tv.Name == c.Args().First() {
						fmt.Printf("Powering off: %s\n", tv.Name)
						if tv.SendCommand("1") {
							fmt.Printf("Powered off %s", tv.Name)
						} else {
							fmt.Printf("%s: Failed\n", tv.Name)
						}
					} else {
						fmt.Printf("Couldn't find tv %s\n", c.Args().First())
					}
				}
			},
		},
	}

	app.Run(os.Args)
}
