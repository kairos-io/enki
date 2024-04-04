package cmd

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kairos-io/enki/pkg/config"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/foxboron/go-uefi/efi/signature"
	efiutil "github.com/foxboron/go-uefi/efi/util"
	"github.com/foxboron/sbctl"
	"github.com/foxboron/sbctl/certs"
	"github.com/foxboron/sbctl/fs"
)

const (
	skipMicrosoftCertsFlag = "skip-microsoft-certs-I-KNOW-WHAT-IM-DOING"
	customCertDirFlag      = "custom-cert-dir"
)

func NewGenkeyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "genkey NAME",
		Short: "Generate secureboot keys under the uuid generated by NAME",
		Args:  cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Set this after parsing of the flags, so it fails on parsing and prints usage properly
			cobraCmd.SilenceUsage = true

			cfg, err := config.ReadConfigBuild(viper.GetString("config-dir"), cobraCmd.Flags())
			if err != nil {
				return err
			}
			l := cfg.Logger
			name := args[0]

			uuid := sbctl.CreateUUID()
			if err != nil {
				return err
			}
			guid := efiutil.StringToGUID(string(uuid))
			output, _ := cobraCmd.Flags().GetString("output")

			err = os.MkdirAll(output, 0700)
			if err != nil {
				l.Errorf("Error creating output directory: %s", err)
				return err
			}

			derDir := ""
			if customCertDir := viper.GetString(customCertDirFlag); customCertDir != "" {
				derDir, err = prepareCustomCertsDir(l)
				if err != nil {
					l.Errorf("Error preparing custom certs directory: %s", err)
					return err
				}
			}

			defer os.RemoveAll(derDir)

			for _, keyType := range []string{"PK", "KEK", "db"} {
				l.Infof("Generating %s", keyType)
				key := filepath.Join(output, fmt.Sprintf("%s.key", keyType))
				pem := filepath.Join(output, fmt.Sprintf("%s.pem", keyType))
				der := filepath.Join(output, fmt.Sprintf("%s.der", keyType))

				args := []string{
					"req", "-nodes", "-x509", "-subj", fmt.Sprintf("/CN=%s/", name),
					"-keyout", key,
					"-out", pem,
				}
				if viper.GetString("expiration-in-days") != "" {
					args = append(args, "-days", viper.GetString("expiration-in-days"))
				}
				cmd := exec.Command("openssl", args...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					l.Errorf("Error generating %s: %s", keyType, string(out))
					return err
				}
				l.Infof("%s generated at %s and %s", keyType, key, pem)

				l.Infof("Converting %s.pem to DER", keyType)
				cmd = exec.Command(
					"openssl", "x509", "-outform", "DER", "-in", pem, "-out", der,
				)

				out, err = cmd.CombinedOutput()
				if err != nil {
					l.Errorf("Error generating %s: %s", keyType, string(out))
					return err
				}
				l.Infof("%s generated at %s", keyType, der)

				err = generateAuthKeys(*guid, output, keyType, derDir)
				if err != nil {
					l.Errorf("Error generating auth keys: %s", err)
					return err
				}
			}

			// Generate the policy encryption key
			l.Infof("Generating policy encryption key")
			tpmPrivate := filepath.Join(output, "tpm2-pcr-private.pem")
			cmd := exec.Command(
				"openssl", "genrsa", "-out", tpmPrivate, "2048",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				l.Errorf("Error generating tpm2-pcr-private.pem: %s", string(out))
				return err
			}
			return nil
		},
	}
	c.Flags().StringP("output", "o", "keys/", "Output directory for the keys")
	c.Flags().StringP("expiration-in-days", "e", "365", "In how many days from today should the certificates expire")
	c.Flags().Bool(skipMicrosoftCertsFlag, false, "When set to true, microsoft certs are not included in the KEK and db files. THIS COULD BRICK YOUR SYSTEM! (https://wiki.archlinux.org/title/Unified_Extensible_Firmware_Interface/Secure_Boot#Enrolling_Option_ROM_digests). Only use this if you are sure your hardware doesn't need the microsoft certs!")

	c.Flags().String(customCertDirFlag, "", "Path to a directory containing custom certificates to enroll")

	viper.BindPFlag("expiration-in-days", c.Flags().Lookup("expiration-in-days"))
	return c
}

func init() {
	rootCmd.AddCommand(NewGenkeyCmd())
}

func generateAuthKeys(guid efiutil.EFIGUID, keyPath, keyType, customDerCertDir string) error {
	// Prepare all the keys we need
	key, err := fs.ReadFile(filepath.Join(keyPath, keyType+".key"))
	if err != nil {
		return fmt.Errorf("reading the key file %w", err)
	}

	pem, err := fs.ReadFile(filepath.Join(keyPath, keyType+".pem"))
	if err != nil {
		return fmt.Errorf("reading the pem file %w", err)
	}

	sigdb := signature.NewSignatureDatabase()

	if err = sigdb.Append(signature.CERT_X509_GUID, guid, pem); err != nil {
		return fmt.Errorf("appending signature %w", err)
	}

	if keyType != "PK" && !viper.GetBool(skipMicrosoftCertsFlag) {
		// Load microsoft certs
		oemSigDb, err := certs.GetOEMCerts("microsoft", keyType)
		if err != nil {
			return fmt.Errorf("failed to load microsoft keys (type %s): %w", keyType, err)
		}
		sigdb.AppendDatabase(oemSigDb)
	}

	// TODO: PK too?
	if keyType != "PK" && customDerCertDir != "" {
		customSigDb, err := certs.GetCustomCerts(customDerCertDir, keyType)
		if err != nil {
			return fmt.Errorf("could not load custom keys (type: %s): %w", keyType, err)
		}
		sigdb.AppendDatabase(customSigDb)
	}

	signedDB, err := sbctl.SignDatabase(sigdb, key, pem, keyType)
	if err != nil {
		return fmt.Errorf("creating the signed db: %w", err)
	}

	if err := fs.WriteFile(filepath.Join(keyPath, keyType+".auth"), signedDB, 0o644); err != nil {
		return fmt.Errorf("writing the auth file: %w", err)
	}

	if err := fs.WriteFile(filepath.Join(keyPath, keyType+".esl"), sigdb.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing the esl file: %w", err)
	}

	return nil
}

// prepareCustomCertsDir takes a cert directory with keys as they are exported
// from the UEFI firmware and prepares them for use with sbctl.
// The keys are exported in the "authenticated variables" format.
// The keys are expected to be in the "der" format in a specific directory structure.
// The given directory should have the following files:
// - db
// - dbx
// - KEK
// - PK
// It returns the prepared temporary directory where the keys are stored in
// "der" format in the expected directories.
func prepareCustomCertsDir(l v1.Logger) (string, error) {
	customCertDir := viper.GetString(customCertDirFlag)
	if customCertDir != "" {
		if _, err := os.Stat(customCertDir); os.IsNotExist(err) {
			return "", fmt.Errorf("custom cert directory does not exist: %s", customCertDir)
		}
	}

	// create a temporary directory to store the custom certs
	tmpDir, err := os.MkdirTemp("", "sbctl-custom-certs-*")
	if err != nil {
		return "", fmt.Errorf("creating temporary directory: %w", err)
	}

	// TODO: PK too?
	for _, keyType := range []string{"db", "KEK"} {
		b, err := ioutil.ReadFile(filepath.Join(customCertDir, keyType))
		if err != nil {
			return "", fmt.Errorf("reading custom cert file %s: %w", keyType, err)
		}
		f := bytes.NewReader(b)
		siglist, err := signature.ReadSignatureDatabase(f)
		if err != nil {
			return "", fmt.Errorf("reading signature database: %w", err)
		}

		l.Infof("Converting custom certs (type: %s)\n", keyType)
		for _, sig := range siglist {
			for _, sigEntry := range sig.Signatures {
				l.Infof("	Signature Owner: %s\n", sigEntry.Owner.Format())
				switch sig.SignatureType {
				case signature.CERT_X509_GUID, signature.CERT_SHA256_GUID:
					cert, _ := x509.ParseCertificate(sigEntry.Data)
					if cert != nil {
						keyDir := filepath.Join(tmpDir, "custom", keyType)
						err := os.MkdirAll(keyDir, 0755)
						if err != nil {
							return "", fmt.Errorf("creating directory for key type %s: %w", keyType, err)
						}
						os.WriteFile(filepath.Join(keyDir, fmt.Sprintf("%s%s", keyType, cert.SerialNumber.String())), cert.Raw, 0644)
					}
				default:
					l.Errorf("Not implemented!\n%s\n", sig.SignatureType.Format())
				}
			}
		}
	}

	return tmpDir, nil
}
