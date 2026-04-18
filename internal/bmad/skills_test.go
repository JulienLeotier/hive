package bmad

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSkillRegistry vérifie la cohérence du registre : chaque skill
// doit avoir un Command /bmad-*, un Scope valide, un Phase et un Name.
// Canari pour les PRs qui ajouteraient un skill sans remplir les
// champs — l'UI dépend de Scope/Phase pour filtrer correctement.
func TestSkillRegistry(t *testing.T) {
	validScopes := map[SkillScope]bool{
		ScopeProject: true,
		ScopeEpic:    true,
		ScopeStory:   true,
	}
	seen := map[string]bool{}
	for _, sk := range SkillRegistry {
		t.Run(sk.Command, func(t *testing.T) {
			assert.True(t, len(sk.Command) > len("/bmad-"),
				"command too short : %q", sk.Command)
			assert.True(t, sk.Command[:6] == "/bmad-",
				"command must start with /bmad- : %q", sk.Command)
			assert.True(t, validScopes[sk.Scope],
				"scope invalide : %q", sk.Scope)
			assert.NotEmpty(t, sk.Name)
			assert.NotEmpty(t, sk.Phase)
			assert.False(t, seen[sk.Command],
				"skill dupliqué : %q", sk.Command)
			seen[sk.Command] = true
		})
	}
	assert.GreaterOrEqual(t, len(SkillRegistry), 10,
		"registre trop petit, au moins les skills principales sont attendues")
}

func TestFindSkill(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		sk := FindSkill("/bmad-code-review")
		if assert.NotNil(t, sk) {
			assert.Equal(t, ScopeStory, sk.Scope)
		}
	})
	t.Run("missing", func(t *testing.T) {
		assert.Nil(t, FindSkill("/bmad-does-not-exist"))
	})
	t.Run("empty", func(t *testing.T) {
		assert.Nil(t, FindSkill(""))
	})
}
