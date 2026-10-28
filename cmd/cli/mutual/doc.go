/*
Package mutual lets CLI plugins mark their flags as mutually exclusive with
flags from other, independent plugins.

As plugins don't know about each other, they cannot directly use cobra's
[cobra.Command.MarkFlagsMutuallyExclusive]. Instead, plugins annotate their
flags using [ExclusiveInGroup] with a group name. After all other CLI plugins
have been set up, this package walks the command tree, gathers the annotated
flags by group, and then marks each group's flags as mutually exclusive.
*/
package mutual
