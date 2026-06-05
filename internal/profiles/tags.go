package profiles

// CanonicalTags is the ordered list of predefined use-case tags shown in the p-panel
// checkbox row. "image" and "audio" are behavioral — they opt a llama/koboldcpp
// profile into --mmproj injection at launch. The remaining tags are informational.
var CanonicalTags = []string{
	"image", "audio", "multimodal",
	"thinking", "reasoning",
	"coding", "agentic", "tool-calling",
	"moe",
}

// CanonicalPrimaries is the ordered list of predefined use-case primary values shown
// in the p-panel checkbox row. Multiple primaries may be selected for a profile.
var CanonicalPrimaries = []UseCasePrimary{
	UseCaseGeneral,
	UseCaseEval,
}
