package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("genkey", func() {
	var resultDir string
	var err error
	var enki *Enki

	BeforeEach(func() {
		resultDir, err = os.MkdirTemp("", "enki-genkey-test-")
		Expect(err).ToNot(HaveOccurred())

		enki = NewEnki("enki-image", resultDir)
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
})

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
