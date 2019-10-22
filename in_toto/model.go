package in_toto

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

/*
KeyVal contains the actual values of a key, as opposed to key metadata such as
a key identifier or key type.  For RSA keys, the key value is a pair of public
and private keys in PEM format stored as strings.  For public keys the Private
field may be an empty string.
*/
type KeyVal struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

/*
Key represents a generic in-toto key that contains key metadata, such as an
identifier, supported hash algorithms to create the identifier, the key type
and the supported signature scheme, and the actual key value.
*/
type Key struct {
	KeyId               string   `json:"keyid"`
	KeyIdHashAlgorithms []string `json:"keyid_hash_algorithms"`
	KeyType             string   `json:"keytype"`
	KeyVal              KeyVal   `json:"keyval"`
	Scheme              string   `json:"scheme"`
}

/*
Signature represents a generic in-toto signature that contains the identifier
of the Key, which was used to create the signature and the signature data.  The
used signature scheme is found in the corresponding Key.
*/
type Signature struct {
	KeyId string `json:"keyid"`
	Sig   string `json:"sig"`
}

/*
Link represents the evidence of a supply chain step performed by a functionary.
It should be contained in a generic Metablock object, which provides
functionality for signing and signature verification, and reading from and
writing to disk.
*/
type Link struct {
	Type        string                 `json:"_type"`
	Name        string                 `json:"name"`
	Materials   map[string]interface{} `json:"materials"`
	Products    map[string]interface{} `json:"products"`
	ByProducts  map[string]interface{} `json:"byproducts"`
	Command     []string               `json:"command"`
	Environment map[string]interface{} `json:"environment"`
}

/*
LinkNameFormat represents a format string used to create the filename for a
signed Link (wrapped in a Metablock). It consists of the name of the link and
the first 8 characters of the signing key id.  LinkNameFormatShort is for links
that are not signed, e.g.:
  fmt.Sprintf(LinkNameFormat, "package",
      "2f89b9272acfc8f4a0a0f094d789fdb0ba798b0fe41f2f5f417c12f0085ff498")
  // returns "package.2f89b9272.link"

  fmt.Sprintf(LinkNameFormatShort, "unsigned")
  // returns "unsigned.link"
*/
const LinkNameFormat = "%s.%.8s.link"
const LinkNameFormatShort = "%s.link"
const SublayoutLinkDirFormat = "%s.%.8s"

/*
SupplyChainItem summarizes common fields of the two available supply chain
item types, Inspection and Step.
*/
type SupplyChainItem struct {
	Name              string     `json:"name"`
	ExpectedMaterials [][]string `json:"expected_materials"`
	ExpectedProducts  [][]string `json:"expected_products"`
}

/*
Inspection represents an in-toto supply chain inspection, whose command in the
Run field is executed during final product verification, generating unsigned
link metadata.  Materials and products used/produced by the inspection are
constrained by the artifact rules in the inspection's ExpectedMaterials and
ExpectedProducts fields.
*/
type Inspection struct {
	Type string   `json:"_type"`
	Run  []string `json:"run"`
	SupplyChainItem
}

/*
Step represents an in-toto step of the supply chain performed by a functionary.
During final product verification in-toto looks for corresponding Link
metadata, which is used as signed evidence that the step was performed
according to the supply chain definition.  Materials and products used/produced
by the step are constrained by the artifact rules in the step's
ExpectedMaterials and ExpectedProducts fields.
*/
type Step struct {
	Type            string   `json:"_type"`
	PubKeys         []string `json:"pubkeys"`
	ExpectedCommand []string `json:"expected_command"`
	Threshold       int      `json:"threshold"`
	SupplyChainItem
}

/*
Layout represents the definition of a software supply chain.  It lists the
sequence of steps required in the software supply chain and the functionaries
authorized to perform these steps.  Functionaries are identified by their
public keys.  In addition, the layout may list a sequence of inspections that
are executed during in-toto supply chain verification.  A layout should be
contained in a generic Metablock object, which provides functionality for
signing and signature verification, and reading from and writing to disk.
*/
type Layout struct {
	Type    string         `json:"_type"`
	Steps   []Step         `json:"steps"`
	Inspect []Inspection   `json:"inspect"`
	Keys    map[string]Key `json:"keys"`
	Expires string         `json:"expires"`
	Readme  string         `json:"readme"`
}

// Go does not allow to pass `[]T` (slice with certain type) to a function
// that accepts `[]interface{}` (slice with generic type)
// We have to manually create the interface slice first, see
// https://golang.org/doc/faq#convert_slice_of_interface
// TODO: Is there a better way to do polymorphism for steps and inspections?
func (l *Layout) StepsAsInterfaceSlice() []interface{} {
	stepsI := make([]interface{}, len(l.Steps))
	for i, v := range l.Steps {
		stepsI[i] = v
	}
	return stepsI
}
func (l *Layout) InspectAsInterfaceSlice() []interface{} {
	inspectionsI := make([]interface{}, len(l.Inspect))
	for i, v := range l.Inspect {
		inspectionsI[i] = v
	}
	return inspectionsI
}

/*
Metablock is a generic container for signable in-toto objects such as Layout
or Link.  It has two fields, one that contains the signable object and one that
contains corresponding signatures.  Metablock also provides functionality for
signing and signature verification, and reading from and writing to disk.
*/
type Metablock struct {
	// NOTE: Whenever we want to access an attribute of `Signed` we have to
	// perform type assertion, e.g. `metablock.Signed.(Layout).Keys`
	// Maybe there is a better way to store either Layouts or Links in `Signed`?
	// The notary folks seem to have separate container structs:
	// https://github.com/theupdateframework/notary/blob/master/tuf/data/root.go#L10-L14
	// https://github.com/theupdateframework/notary/blob/master/tuf/data/targets.go#L13-L17
	// I implemented it this way, because there will be several functions that
	// receive or return a Metablock, where the type of Signed has to be inferred
	// on runtime, e.g. when iterating over links for a layout, and a link can
	// turn out to be a layout (sublayout)
	Signed     interface{} `json:"signed"`
	Signatures []Signature `json:"signatures"`
}

/*
Load parses JSON formatted metadata at the passed path into the Metablock
object on which it was called.  It returns an error if it cannot parse
a valid JSON formatted Metablock that contains a Link or Layout.
*/
func (mb *Metablock) Load(path string) error {
	// Open file and close before returning
	jsonFile, err := os.Open(path)
	defer jsonFile.Close()
	if err != nil {
		return err
	}

	// Read entire file
	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	// Unmarshal JSON into a map of raw messages (signed and signatures)
	// We can't fully unmarshal immediately, because we need to inspect the
	// type (link or layout) to decide which data structure to use
	var rawMb map[string]*json.RawMessage
	if err := json.Unmarshal(jsonBytes, &rawMb); err != nil {
		return err
	}

	// Error out on missing `signed` or `signatures` field or if
	// one of them has a `null` value, which would lead to a nil pointer
	// dereference in Unmarshal below.
	if rawMb["signed"] == nil || rawMb["signatures"] == nil {
		return fmt.Errorf("In-toto metadata requires 'signed' and" +
			" 'signatures' parts")
	}

	// Fully unmarshal signatures part
	if err := json.Unmarshal(*rawMb["signatures"], &mb.Signatures); err != nil {
		return err
	}

	// Temporarily copy signed to opaque map to inspect the `_type` of signed
	// and create link or layout accordingly
	var signed map[string]interface{}
	if err := json.Unmarshal(*rawMb["signed"], &signed); err != nil {
		return err
	}

	if signed["_type"] == "link" {
		var link Link
		if err := json.Unmarshal(*rawMb["signed"], &link); err != nil {
			return err
		}
		mb.Signed = link

	} else if signed["_type"] == "layout" {
		var layout Layout
		if err := json.Unmarshal(*rawMb["signed"], &layout); err != nil {
			return err
		}
		mb.Signed = layout

	} else {
		return fmt.Errorf("The '_type' field of the 'signed' part of in-toto" +
			" metadata must be one of 'link' or 'layout'")
	}

	return nil
}

/*
Dump JSON serializes and writes the Metablock on which it was called to the
passed path.  It returns an error if JSON serialization or writing fails.
*/
func (mb *Metablock) Dump(path string) error {
	// JSON encode Metablock formatted with newlines and indentation
	// TODO: parametrize format
	jsonBytes, err := json.MarshalIndent(mb, "", "  ")
	if err != nil {
		return err
	}

	// Write JSON bytes to the passed path with permissions (-rw-r--r--)
	err = ioutil.WriteFile(path, jsonBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

/*
GetSignableRepresentation returns the canonical JSON representation of the
Signed field of the Metablock on which it was called.  If canonicalization
fails the first return value is nil and the second return value is the error.
*/
func (mb *Metablock) GetSignableRepresentation() ([]byte, error) {
	return encodeCanonical(mb.Signed)
}

/*
VerifySignature verifies the first signature, corresponding to the passed Key,
that it finds in the Signatures field of the Metablock on which it was called.
It returns an error if Signatures does not contain a Signature corresponding to
the passed Key, the object in Signed cannot be canonicalized, or the Signature
is invalid.
*/
func (mb *Metablock) VerifySignature(key Key) error {
	var sig Signature
	for _, s := range mb.Signatures {
		if s.KeyId == key.KeyId {
			sig = s
			break
		}
	}

	if sig == (Signature{}) {
		return fmt.Errorf("No signature found for key '%s'", key.KeyId)
	}

	dataCanonical, err := mb.GetSignableRepresentation()
	if err != nil {
		return err
	}

	if err := VerifySignature(key, sig, dataCanonical); err != nil {
		return err
	}
	return nil
}

/*
Sign signs the signed portion of the metablock using the Key object provided.
It then appends the resulting signature to the signatures field as provided.
It return an error if the Signed object cannot be canonicalized, or if the key
is invalid or not supported.
*/
func (mb *Metablock) Sign(key Key) error {

	dataCanonical, err := mb.GetSignableRepresentation()
	if err != nil {
		return err
	}
	var newSignature Signature

	// FIXME: we could be fancier about signature-generation using a dispatch
	// table or something but for now let's just be explicit
	// (also, lolnogenerics)
	if key.KeyType == "ed25519" && key.Scheme == "ed25519" {
		newSignature, err = generateEd25519Signature(dataCanonical, key)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("This key type or signature scheme is not supported yet!")
	}

	mb.Signatures = append(mb.Signatures, newSignature)

	return nil
}
