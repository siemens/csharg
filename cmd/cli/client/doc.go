/*
Package client provides a modular plugin infrastructure for creating packetflix
capture clients based on command configuration.

# Getting a Client (Consumers)

Commands consuming a capture service client get a suitable client by calling
[NewSharkTank] with their command:

	st, err := client.NewSharkTank(cmd)

This asks the registered factories one after another and returns the first
client created, or the first error encountered.

# Adding Clients (Providers)

Client providers add a new type of client by registering a [New] factory
function in their package's init function:

	func init() {
		plugger.Group[client.New]().Register(
			newClient, plugger.WithPlugin("myclient"))
	}

The factory inspects the CLI flags of the passed command and returns a nil
client and nil error when it isn't responsible. Flags that select a client
endpoint should be marked as mutually exclusive in the [MutuallyExclusiveGroup]
group using [mutual.ExclusiveInGroup].

[mutual.ExclusiveInGroup]: https://pkg.go.dev/github.com/siemens/csharg/v2/cmd/cli/mutual#ExclusiveInGroup
*/
package client
