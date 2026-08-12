// SPDX-License-Identifier: Apache-2.0

package sando

// TrustedHTML is HTML whose author has deliberately accepted responsibility
// for its safety. Its contents are opaque outside this package.
type TrustedHTML struct{ value string }

// TrustHTML marks value as deliberately trusted HTML. The caller is responsible
// for supplying a balanced fragment that is valid at an HTML content boundary
// and does not leave the parser in a different context. Prefer ordinary values
// and WriteText whenever possible.
func TrustHTML(value string) TrustedHTML { return TrustedHTML{value: value} }

// TrustedURL is a URL whose author has deliberately accepted responsibility
// for its scheme and navigation behavior. HTML attribute syntax is still
// escaped when the value is written.
type TrustedURL struct{ value string }

// TrustURL marks value as a deliberately trusted URL, bypassing the ordinary
// URL scheme allowlist. It does not bypass HTML attribute escaping.
func TrustURL(value string) TrustedURL { return TrustedURL{value: value} }

// TrustedJS is JavaScript whose author has deliberately accepted
// responsibility for its safety. Its contents are opaque outside this package.
type TrustedJS struct{ value string }

// TrustJS marks value as deliberately trusted JavaScript. The caller is also
// responsible for preventing an HTML </script sequence from terminating its
// container. Plain strings cannot be passed to WriteJS.
func TrustJS(value string) TrustedJS { return TrustedJS{value: value} }

// TrustedCSS is CSS whose author has deliberately accepted responsibility for
// its safety. Its contents are opaque outside this package.
type TrustedCSS struct{ value string }

// TrustCSS marks value as deliberately trusted CSS. The caller is also
// responsible for preventing an HTML </style sequence from terminating its
// container. Plain strings cannot be passed to WriteCSS.
func TrustCSS(value string) TrustedCSS { return TrustedCSS{value: value} }
