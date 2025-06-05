package cryptocmd

import (
	"fmt"

	"github.com/Mattddixo/dsp/internal/crypto"
	"github.com/urfave/cli/v2"
)

// Command returns the crypto command
func Command() *cli.Command {
	return &cli.Command{
		Name:  "crypto",
		Usage: "Manage encryption keys and recipients",
		Description: `Manage encryption keys and recipients for secure bundle sharing.

The crypto system uses age encryption (https://age-encryption.org) for secure file encryption.
It supports both public key encryption and passphrase-based encryption.

Commands:
  init            Initialize the crypto system and generate a new key pair
  add-recipient   Add a new recipient's public key
  list-recipients List all registered recipients
  remove-recipient Remove a recipient
  export-key      Export your public key

Examples:
  # Initialize the crypto system
  dsp crypto init

  # Add a recipient's public key
  dsp crypto add-recipient --name "alice" --key "age1..."

  # List all recipients
  dsp crypto list-recipients

  # Remove a recipient
  dsp crypto remove-recipient --name "alice"

  # Export your public key
  dsp crypto export-key

For more information about a specific command, use:
  dsp crypto <command> --help`,
		Subcommands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize crypto system and generate key pair",
				Description: `Initialize the crypto system and generate a new key pair.

This command will:
1. Create the global crypto directory (~/.dsp-global by default)
2. Generate a new age key pair
3. Store the private key securely
4. Display your public key for sharing

The generated keys will be used for encrypting bundles when encryption is enabled.`,
				Action: func(c *cli.Context) error {
					manager, err := crypto.NewKeyManager()
					if err != nil {
						return fmt.Errorf("failed to create key manager: %w", err)
					}

					if err := manager.InitializeKeys(); err != nil {
						return fmt.Errorf("failed to initialize crypto system: %w", err)
					}

					publicKey, err := manager.GetPublicKey()
					if err != nil {
						return fmt.Errorf("failed to get public key: %w", err)
					}

					fmt.Println("Crypto system initialized successfully!")
					fmt.Println("\nYour public key:")
					fmt.Println(publicKey)
					fmt.Println("\nKeep your private key secure at:", manager.GetPrivateKeyPath())
					return nil
				},
			},
			{
				Name:  "add-recipient",
				Usage: "Add a new recipient",
				Description: `Add a new recipient's public key to the system.

This command adds a recipient's public key to your list of trusted recipients.
When encryption is enabled, bundles can be encrypted for specific recipients.

The recipient's public key should be in age format (starts with "age1...").`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "Name of the recipient (e.g., 'alice', 'bob')",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "key",
						Usage:    "Public key of the recipient (in age format, starts with 'age1...')",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					manager, err := crypto.NewKeyManager()
					if err != nil {
						return fmt.Errorf("failed to create key manager: %w", err)
					}

					if err := manager.AddRecipient(c.String("name"), c.String("key")); err != nil {
						return fmt.Errorf("failed to add recipient: %w", err)
					}

					fmt.Printf("Added recipient '%s' successfully!\n", c.String("name"))
					return nil
				},
			},
			{
				Name:  "list-recipients",
				Usage: "List all recipients",
				Description: `List all registered recipients and their public keys.

This command displays all recipients that have been added to your system,
including their names and public keys. Use this to verify your recipients
or to share your list of trusted recipients.`,
				Action: func(c *cli.Context) error {
					manager, err := crypto.NewKeyManager()
					if err != nil {
						return fmt.Errorf("failed to create key manager: %w", err)
					}

					recipients := manager.ListRecipients()
					if len(recipients) == 0 {
						fmt.Println("No recipients found.")
						return nil
					}

					fmt.Println("Recipients:")
					for _, r := range recipients {
						fmt.Printf("\nName: %s\n", r.Name)
						fmt.Printf("Key: %s\n", r.Key)
					}
					return nil
				},
			},
			{
				Name:  "remove-recipient",
				Usage: "Remove a recipient",
				Description: `Remove a recipient from your list of trusted recipients.

This command removes a recipient's public key from your system.
After removal, you will no longer be able to encrypt bundles for this recipient.`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "Name of the recipient to remove",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					manager, err := crypto.NewKeyManager()
					if err != nil {
						return fmt.Errorf("failed to create key manager: %w", err)
					}

					if err := manager.RemoveRecipient(c.String("name")); err != nil {
						return fmt.Errorf("failed to remove recipient: %w", err)
					}

					fmt.Printf("Removed recipient '%s' successfully!\n", c.String("name"))
					return nil
				},
			},
			{
				Name:  "export-key",
				Usage: "Export your public key",
				Description: `Export your public key for sharing with others.

This command displays your public key in age format. Share this key with
others so they can add you as a recipient and encrypt bundles for you.

The key is displayed in a format that can be directly used with the
add-recipient command.`,
				Action: func(c *cli.Context) error {
					manager, err := crypto.NewKeyManager()
					if err != nil {
						return fmt.Errorf("failed to create key manager: %w", err)
					}

					publicKey, err := manager.GetPublicKey()
					if err != nil {
						return fmt.Errorf("failed to get public key: %w", err)
					}

					fmt.Println(publicKey)
					return nil
				},
			},
		},
	}
}
