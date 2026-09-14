// Package gen generates the fill-in-the-blank sentences a validated word
// list plays with (brief/ENCRE_04 §9, ENCRE_03 §9).
//
// A sentence is asked for one word at a time, from the Anthropic Messages
// API behind [Client] — [AnthropicClient] is the real implementation,
// reached over plain net/http rather than the official SDK (see
// [AnthropicClient]'s doc comment for why); anything else satisfying
// [Client] is what every test in this package uses instead, so none of them
// ever makes a network call.
//
// The API is trusted for nothing: [Filter] rejects any returned sentence
// longer than eight words or containing a word outside the embedded CE1
// [Whitelist], and [GenerateSentences] never panics on a malformed
// response — it returns an error, the same way it does for a network
// failure. Either one leaves the caller exactly where ENCRE_03 §9 says a
// parent must always be able to land: typing the sentence themselves. This
// package never blocks on that fallback; it is up to the caller
// ([github.com/oioio-space/encre/server/api]) to offer it.
package gen
