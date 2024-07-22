package netconf_test

import (
	"context"
	"log"
	"time"
	"os"
	"fmt"
	"testing"

	"github.com/kenichiogaki/netconf"
	ncssh "github.com/nemith/netconf/transport/ssh"
	"golang.org/x/crypto/ssh"
	"encoding/xml"
)

//The followings are the definition of struct corresponding to opencofig interface.yang model with struct field tags
type AddressConfig struct {
     XMLName	   xml.Name	`xml:"config"`
     Ip		   string	`xml:"ip"`
     PrefixLength  int		`xml:"prefix-length"`
}

type Address struct {
     XMLName xml.Name	`xml:"address"`
     Ip	     string	`xml:"ip"`
     Config  AddressConfig
}

type Addresses struct {
     XMLName   xml.Name		`xml:"addresses"`
     Addresses []Address	`xml:"address"`
}

type Ipv4 struct {
     XMLName	 xml.Name	`xml:"http://openconfig.net/yang/interfaces/ip ipv4"`
     Addresses	 Addresses
}

type SubInterface struct {
     XMLName	  xml.Name	`xml:"subinterface"`
     Index	  int		`xml:"index"`
     Ipv4	  Ipv4
}

type Type struct {
     XMLName	 xml.Name	`xml:"type"`
     Value	 string		`xml:",chardata"`
     Idx	 string		`xml:"xmlns:idx,attr"`
}

type Config struct {
     XMLName	   xml.Name	`xml:"config"`
     Name	   string	`xml:"name"`
     Type	   Type
     Enabled	   bool		`xml:"enabled"`
}

type EthernetConfig struct {
     XMLName	    xml.Name	`xml:"config"`
     AutoNegotiate  bool	`xml:"auto-negotiate"`
}

type Ethernet struct {
     XMLName  xml.Name		`xml:"http://openconfig.net/yang/interfaces/ethernet ethernet"`
     Config   EthernetConfig
}

type SubInterfaces struct {
     XMLName	   xml.Name		`xml:"subinterfaces"`
     SubInterfaces []SubInterface	`xml:"subinterface"`
}

type Interface struct {
     XMLName	      xml.Name		`xml:"interface"`
     Name	      string		`xml:"name"`
     Config	      Config
     Ethernet	      Ethernet
     SubInterfaces    SubInterfaces
}

type Interfaces struct {
     XMLName	xml.Name	`xml:"http://openconfig.net/yang/interfaces interfaces"`
     Interfaces	[]Interface	`xml:"interface"`
}

const sshAddr = "172.18.0.50:22"	//put your netconf device's IP address 

func TestSSH(t *testing.T) {
     //example interface struct instance
     var subInterfaceData []SubInterface
     for i := 0; i < 2; i++ {
	 addressString := fmt.Sprintf("10.0.%d.1", i)
	 addressData := Address{
		     Ip: addressString,
		     Config: AddressConfig{
			     Ip: addressString,
			     PrefixLength: 24,
		     },
	 }
	 subInterfaceData = append(subInterfaceData,
			  SubInterface{
				Index: i,
				Ipv4: Ipv4{
				      Addresses: Addresses{
						 Addresses: []Address{addressData},
				      },
				},
	})
     }
     interfaceData := Interface{
		   Name: "GigabitEthernet0/0/0/0",
		   Config: Config{
			   Name: "GigabitEthernet0/0/0/0",
			   Type: Type{
				 Value: "idx:ethernetCsmacd",
				 Idx: "urn:ietf:params:xml:ns:yang:iana-if-type",
			   },
		   Enabled: true,
		   },
		   Ethernet: Ethernet{
			     Config: EthernetConfig{
				     AutoNegotiate: false,
			     },
		   },
		   SubInterfaces: SubInterfaces{
				  SubInterfaces: subInterfaceData,
		   },
     }
     interfacesData := Interfaces{
		    Interfaces: []Interface{interfaceData},
     }

     config := &ssh.ClientConfig{
	    User: "cisco",			//put your netconf device's user acccount 
	    Auth: []ssh.AuthMethod{
		  ssh.Password("cisco123"),	//put your netconf device's passowrd
	    },
	    HostKeyCallback: ssh.InsecureIgnoreHostKey(),
     }
     ctx := context.Background()
     ctx1, cancel := context.WithTimeout(ctx, 5*time.Second)
     defer cancel()

     transport, err := ncssh.Dial(ctx1, "tcp", sshAddr, config)
     if err != nil {
	panic(err)
     }
     defer transport.Close()

     session, err := netconf.Open(transport)
     if err != nil {
	panic(err)
     }

     // timeout for the call itself.
     ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
     defer cancel()

     //example filter definition
     filterXml := `
     	       <interfaces xmlns="http://openconfig.net/yang/interfaces">
	           <interface>
		       <state>
		           <name>GigabitEthernet0/0/0/0</name>
		       </state>			
	       	   </interface>
	       </interfaces>
	       `
//     deviceConfig, err := session.GetConfig(ctx2, "running")
//     deviceConfig, err := session.GetConfig(ctx2, "running", filterXml)
     deviceConfig, err := session.Get(ctx2, filterXml)

     if err != nil {
	log.Printf("failed to get config: %v", err)
     }
/*
     var xmlResp = Interfaces{}
     err = xml.Unmarshal([]byte(deviceConfig), &xmlResp)
     if err != nil {
     	log.Printf("xml.Unmarshal err: %v", err)
     }
     log.Printf("xmlResp :%+v\n", xmlResp)
*/
     if err != nil {
	log.Fatalf("failed to get config: %v", err)
     }

     log.Printf("Config:\n%s\n", deviceConfig)
     err = os.WriteFile("getconfig.xml", deviceConfig, 0644)
     if err != nil {
	log.Print(err)
     }

     ctx3, cancel := context.WithTimeout(ctx, 5*time.Second)
     defer cancel()
     if err := session.Close(ctx3); err != nil {
	log.Print(err)
     }

     time.Sleep(1 * time.Second)
     transport, err = ncssh.Dial(ctx3, "tcp", sshAddr, config)
     if err != nil {
	panic(err)
     }
     defer transport.Close()
     session, err = netconf.Open(transport)
     if err != nil {
	panic(err)
     }

     //Marshal Interface struct to XML
     xmlBytes, err := xml.MarshalIndent(interfacesData, "", "  ")
     if err != nil {
	log.Fatalf("error: %v", err)
     }

     err = os.WriteFile("editconfig.xml", xmlBytes, 0644)
     if err != nil {
	log.Print(err)
     }

     ctx4, cancel := context.WithTimeout(ctx, 5*time.Second)
     defer cancel()
     err = session.EditConfig(ctx4, "candidate", xmlBytes)
     if err != nil {
	log.Fatalf("failed to edit config: %v", err)
     }

     ctx5, cancel := context.WithTimeout(ctx, 1*time.Second)
     defer cancel()

     //IOS-XR doesn't reply <ok> after receiving <commit/> rpc.
     err = session.Commit(ctx5)
     if err != nil {
	log.Fatalf("failed to commit: %v", err)
     }

     if err := session.Close(context.Background()); err != nil {
	log.Print(err)
     }
}
