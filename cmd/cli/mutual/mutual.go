// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package mutual

// Flag annotation for grouping mutually exclusive flags. Due to the open-ended
// plugin architecture of csharg we cannot directly use cobra's
// MarkFlagsMutuallyExclusive in plugins, but instead such plugins need to
// annotate their flags and we then gather the groups with their flag members in
// order to issue MarkFlagsMutuallyExclusive as necessary.
const MutualFlagGroupAnnotation = "mutually-exclusive-group"
