package managementSessions

// usage:
// go run Connect_netconf.go  -s 192.168.4.64 -g >> output.xml
// go run Connect_netconf.go  -s 192.168.4.64 -e -f config.xml

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log/slog"
	"os"

	//"github.com/beevik/etree"
	"github.com/openshift-telco/go-netconf-client/netconf"
	"github.com/openshift-telco/go-netconf-client/netconf/message"
	"golang.org/x/crypto/ssh"
)

// createSession connects to a NETCONF server and returns the session.
func CreateSession(host, user, pass string) (*netconf.Session, error) {
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	address := fmt.Sprintf("%s:830", host)

	session, err := netconf.NewSessionFromSSHConfig(address, sshConfig, netconf.WithSessionLogger(logger))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	err = session.SendHello(&message.Hello{
		Capabilities: netconf.DefaultCapabilities,
	})
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to send hello: %w", err)
	}

	return session, nil
}

// DefaultNetconfTimeout defines the default timeout in seconds for NETCONF SyncRPC calls.
const DefaultNetconfTimeout int32 = 5

// GetRunningConfig retrieves the <running> config using a <get-config> RPC.
func GetRunningConfig(session *netconf.Session, timeoutSec ...int32) (string, error) {
	timeout := DefaultNetconfTimeout
	if len(timeoutSec) > 0 && timeoutSec[0] > 0 {
		timeout = timeoutSec[0]
	}

	rpc := message.NewGetConfig(message.DatastoreRunning, "", "")
	reply, err := session.SyncRPC(rpc, timeout)
	if err != nil {
		return "", fmt.Errorf("RPC failed (timeout: %ds): %w", timeout, err)
	}

	if reply == nil || reply.RawReply == "" {
		return "", fmt.Errorf("empty reply from device")
	}

	return reply.RawReply, nil
}

// EditConfig sends an <edit-config> RPC with the provided XML payload to the <running> datastore.
func EditConfig(session *netconf.Session, xmlData string, timeoutSec ...int32) error {
	timeout := DefaultNetconfTimeout
	if len(timeoutSec) > 0 && timeoutSec[0] > 0 {
		timeout = timeoutSec[0]
	}

	//fmt.Printf("[NETCONF EditConfig] Sending XML Payload:\n%s\n", xmlData)
	rpc := message.NewEditConfig(
		message.DatastoreRunning,
		message.DefaultOperationTypeMerge,
		xmlData,
	)

	// fmt.Println(prettyPrintXML(xmlData))
	reply, err := session.SyncRPC(rpc, timeout)
	if err != nil {
		return fmt.Errorf("edit-config RPC failed (timeout: %ds): %w", timeout, err)
	}

	if reply == nil || reply.RawReply == "" {
		return fmt.Errorf("empty reply from edit-config")
	}

	if err := checkNetconfOKReply(reply.RawReply); err != nil {
		return fmt.Errorf("edit-config failed: %w", err)
	}

	// fmt.Println("edit-config reply:")
	// fmt.Println(reply.RawReply)

	return nil
}

// loadXMLFromFile reads an XML file from disk and returns it as a string.
func loadXMLFromFile(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read XML file: %w", err)
	}
	return string(data), nil
}

// function used to check if the NETCONF reply contains an <ok/> element, indicating success.
func checkNetconfOKReply(rawReply string) error {

	var rpcReply struct {
		OK       *struct{} `xml:"ok"`
		RPCError *struct {
			ErrorType     string `xml:"error-type"`
			ErrorTag      string `xml:"error-tag"`
			ErrorSeverity string `xml:"error-severity"`
			ErrorMessage  string `xml:"error-message"`
			ErrorInfo     string `xml:"error-info"`
			ErrorPath     string `xml:"error-path"`
		} `xml:"rpc-error"`
	}

	if err := xml.Unmarshal(
		[]byte(rawReply),
		&rpcReply,
	); err != nil {
		return fmt.Errorf(
			"failed parsing NETCONF reply: %w (raw reply: %s)",
			err,
			rawReply,
		)
	}

	if rpcReply.OK == nil {
		if rpcReply.RPCError != nil {
			return fmt.Errorf(
				"NETCONF rpc-error [tag: %s, type: %s, severity: %s]: %s (path: %s, info: %s) [raw: %s]",
				rpcReply.RPCError.ErrorTag,
				rpcReply.RPCError.ErrorType,
				rpcReply.RPCError.ErrorSeverity,
				rpcReply.RPCError.ErrorMessage,
				rpcReply.RPCError.ErrorPath,
				rpcReply.RPCError.ErrorInfo,
				rawReply,
			)
		}
		return fmt.Errorf(
			"NETCONF reply does not contain <ok/> (raw reply: %s)",
			rawReply,
		)
	}

	return nil
}

/*func prettyPrintXML(xmlData string) string {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlData); err != nil {
		return xmlData
	}

	doc.Indent(2)
	formatted, err := doc.WriteToString()
	if err != nil {
		return xmlData
	}

	return formatted
}*/
