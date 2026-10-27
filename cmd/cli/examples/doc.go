/*
Package examples provides a modular plugin infrastructure for illustrating
particular (sub) commands with examples.

# Adding Examples (Providers)

Plugins contribute examples by registering an [Illustrate] function in their
package's init function. It returns a [ForCommands] map from command names to
example texts, so a single plugin can contribute examples to multiple commands:

	func init() {
		plugger.Group[examples.Illustrate]().Register(
			func() examples.ForCommands {
				return examples.ForCommands{
					"list": "# List all capture targets.\ncsharg list",
				}
			},
			plugger.WithPlugin("myplugin"))
	}

Examples from different plugins are separated by empty lines and appear in
plugin order; use [plugger.WithPlacement] to control the position.

# Using Examples (Consumers)

After all other CLI setup plugins have run, this package automatically sets
the examples of the root command's direct sub-commands. To get the combined
examples for a particular command, call [For] with the command's name.
*/
package examples
