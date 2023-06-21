/*
Package cli defines plugin extension points for the csharg command. This
allows to build extended capture service CLI clients that leverage the existing
base implementation.

# Extension Points

The following plugin “group” extension points are available (and also invoked in
this general order):

  - [SetupCLI]: for adding (sub) commands and CLI args to the (in [cobra]
    parlance) “root” command.
  - [CommandExamples]: for adding (more) examples to particular commands, namely
    the “list” and “capture” commands. These plugin functions are invoked after
    all [SetupCLI] plugins have been called, so that all commands have been
    registered by the time the examples should be extended with even more
    examples.
  - [BeforeCommand]: for checking and doing things just before the command runs.
  - [NewClient]: for creating a suitable capture service client, depending on
    CLI args.

Simply put, the plugin mechanism used in csharg is compile-time only and allows
so-called plugins to register functions (and interface implementations) in what
is termed “groups”. The registered functions/interfaces then can be iterated
over. Additionally, the plugin mechanism allows control over the ordering of
plugins: for instance, this allows to register command examples to be picked up
after the csharg base examples. For more details about the plugin mechanism,
please refer to [go-plugger].

[cobra]: https://github.com/spf13/cobra
[go-plugger]: https://github.com/thediveo/go-plugger
*/
package cli
