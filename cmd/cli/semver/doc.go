/*
Package semver allows plugins to override the semantic version reported by the
CLI binary, such as in derived builds.

# Resolving the Version (Consumers)

Commands get the effective semantic version by calling [Resolve] with the
preset version to fall back to:

	version := semver.Resolve(csharg.SemVersion)

# Overriding the Version (Providers)

Plugins override the version by registering an [Override] function in their
package's init function:

	func init() {
		plugger.Group[semver.Override]().Register(
			func() string { return "1.2.3-acme" },
			plugger.WithPlugin("acme"))
	}

Returning an empty string leaves the decision to other plugins, or finally the
preset. The first plugin in plugin order returning a non-empty version wins.
*/
package semver
