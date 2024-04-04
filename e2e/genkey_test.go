package e2e_test

import (
	"bytes"
	"crypto/x509"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/foxboron/go-uefi/efi/signature"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("genkey", func() {
	var resultDir string
	var err error
	var enki *Enki
	var asusKeysDir string

	BeforeEach(func() {
		resultDir, err = os.MkdirTemp("", "enki-genkey-test-")
		Expect(err).ToNot(HaveOccurred())

		currentDir, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		asusKeysDir = filepath.Join(currentDir, "assets", "asus-PN64-vendor-keys")

		enki = NewEnki("enki-image", resultDir, asusKeysDir)
	})

	AfterEach(func() {
		os.RemoveAll(resultDir)
		enki.Cleanup()
	})

	When("expiration-in-days is not specified", func() {
		It("builds certificates with expiration in 365 days", func() {
			out, err := enki.Run("genkey", "-o", resultDir, "mykey")
			Expect(err).ToNot(HaveOccurred(), out)

			expectExpirationIn(365, resultDir)
		})
	})

	When("expiration-in-days is specified", func() {
		It("builds certificates that expire after the specified days", func() {
			out, err := enki.Run("genkey", "-o", resultDir, "-e", "1000", "mykey")
			Expect(err).ToNot(HaveOccurred(), out)

			expectExpirationIn(1000, resultDir)
		})
	})

	When("skip-microsoft-certs-I-KNOW-WHAT-IM-DOING is set", func() {
		It("doesn't bake-in the microsoft certificates", func() {
			out, err := enki.Run("genkey",
				"--skip-microsoft-certs-I-KNOW-WHAT-IM-DOING",
				"-o", resultDir, "mykey")
			Expect(err).ToNot(HaveOccurred(), out)

			expectAuthToNotContainSigner("Microsoft", filepath.Join(resultDir, "db.auth"))
		})
	})

	When("skip-microsoft-certs-I-KNOW-WHAT-IM-DOING is not set", func() {
		It("bakes-in the microsoft certificates", func() {
			out, err := enki.Run("genkey",
				"-o", resultDir, "mykey")
			Expect(err).ToNot(HaveOccurred(), out)

			expectAuthToContainSigner("Microsoft", filepath.Join(resultDir, "db.auth"))
		})
	})

	When("custom-cert-dir is used", func() {
		It("embeds the custom certs", func() {
			out, err := enki.Run("genkey",
				"-o", resultDir,
				"--custom-cert-dir", asusKeysDir, "mykey")
			Expect(err).ToNot(HaveOccurred(), out)

			expectAuthToContainSigner("Microsoft", filepath.Join(resultDir, "db.auth"))
			expectAuthToContainSigner("ASUS", filepath.Join(resultDir, "db.auth"))
			expectAuthToContainSigner("mykey", filepath.Join(resultDir, "db.auth"))
		})
	})
})

func expectAuthToContainSigner(owner, authFile string) {
	Expect(authIssuers(authFile)).To(ContainElement(MatchRegexp(owner)), "Expected %s to be in %s", owner, authFile)
}

func expectAuthToNotContainSigner(owner, authFile string) {
	Expect(authIssuers(authFile)).ToNot(ContainElement(MatchRegexp(owner)), "Expected %s to not be in %s", owner, authFile)
}

func authIssuers(authFile string) []string {
	b, err := ioutil.ReadFile(authFile)
	Expect(err).ToNot(HaveOccurred())

	f := bytes.NewReader(b)
	_, err = signature.ReadEFIVariableAuthencation2(f)
	Expect(err).ToNot(HaveOccurred())
	siglist, err := signature.ReadSignatureDatabase(f)
	Expect(err).ToNot(HaveOccurred())

	issuers := []string{}
	for _, sig := range siglist {
		for _, sigEntry := range sig.Signatures {
			if sig.SignatureType == signature.CERT_X509_GUID {
				cert, _ := x509.ParseCertificate(sigEntry.Data)
				if cert != nil {
					issuers = append(issuers, cert.Issuer.String())
				}
			}
		}
	}

	return issuers
}

// getDateFromString accepts a date in the form: "Feb  6 15:53:30 2025 GMT"
// and returns the day, month and year as integers
func getDateFromString(dateString string) (int, int, int) {
	// Define the layout matching the format of the string
	layout := "Jan  2 15:04:05 2006 MST"
	dateTime, err := time.Parse(layout, dateString)
	Expect(err).ToNot(HaveOccurred())

	return dateTime.Day(), int(dateTime.Month()), dateTime.Year()
}

func expectExpirationIn(n int, resultDir string) {
	By("checking the expiration")
	cmd := exec.Command("openssl", "x509", "-enddate", "-noout",
		"-in", filepath.Join(resultDir, "db.pem"))
	o, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), o)

	dateStr := strings.TrimSpace(strings.TrimPrefix(string(o), "notAfter="))
	certDay, certMonth, certYear := getDateFromString(dateStr)

	expectedTime := time.Now().Add(time.Duration(n) * 24 * time.Hour)
	Expect(certDay).To(Equal(expectedTime.Day()))
	Expect(certMonth).To(Equal(int(expectedTime.Month())))
	Expect(certYear).To(Equal(expectedTime.Year()))
}
