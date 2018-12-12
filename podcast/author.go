package podcast

import "encoding/xml"

// Author represents a named author and email.
//
// For iTunes compliance, both Name and Email are required.
type Author struct {
	XMLName xml.Name `xml:"owner"`
	Name    string   `xml:"name"`
	Email   string   `xml:"email"`
}
