package project

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreCreateAndGet : Create insère en DB, GetByID retourne le
// projet avec son arbre vide (pas d'epic encore). Fondamental pour
// l'intake — si Create casse, aucun projet ne peut être démarré.
func TestStoreCreateAndGet(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	store := NewStore(st.DB)

	ctx := context.Background()
	p, err := store.Create(ctx, "default", "une idée simple", CreateOpts{
		Name:    "demo",
		Workdir: "/tmp/demo",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "demo", p.Name)
	assert.Equal(t, "une idée simple", p.Idea)
	assert.Equal(t, "/tmp/demo", p.Workdir)
	assert.Equal(t, StatusDraft, p.Status)
	assert.Equal(t, "default", p.TenantID)
	assert.False(t, p.Paused)

	// GetByID retourne le même.
	got, err := store.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
	assert.Equal(t, p.Name, got.Name)
	assert.Empty(t, got.Epics, "pas d'epics tant que l'architect n'a pas tourné")
}

func TestStoreCreateRequiresIdea(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	store := NewStore(st.DB)

	_, err = store.Create(context.Background(), "default", "", CreateOpts{})
	assert.Error(t, err)
}

func TestStoreCreateFallbackName(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	store := NewStore(st.DB)

	p, err := store.Create(context.Background(), "", "une idée sans nom fournie", CreateOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, p.Name, "un Name doit être auto-généré depuis l'idée")
	assert.Equal(t, "default", p.TenantID, "tenant vide fallback sur default")
}

func TestStoreList(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	store := NewStore(st.DB)
	ctx := context.Background()

	// Trois projets, deux tenants différents.
	_, err = store.Create(ctx, "tenant-a", "idée A1", CreateOpts{})
	require.NoError(t, err)
	_, err = store.Create(ctx, "tenant-a", "idée A2", CreateOpts{})
	require.NoError(t, err)
	_, err = store.Create(ctx, "tenant-b", "idée B1", CreateOpts{})
	require.NoError(t, err)

	listA, err := store.List(ctx, "tenant-a", 10)
	require.NoError(t, err)
	assert.Len(t, listA, 2, "un tenant ne doit voir que ses projets")

	listAll, err := store.List(ctx, "", 10)
	require.NoError(t, err)
	assert.Len(t, listAll, 3, "tenant vide liste tous les projets")

	listLimit, err := store.List(ctx, "", 1)
	require.NoError(t, err)
	assert.Len(t, listLimit, 1, "limit doit être respecté")
}

func TestStoreUpdateStatus(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	store := NewStore(st.DB)
	ctx := context.Background()

	p, err := store.Create(ctx, "default", "x", CreateOpts{})
	require.NoError(t, err)

	require.NoError(t, store.UpdateStatus(ctx, p.ID, StatusBuilding))
	got, err := store.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusBuilding, got.Status)
}

func TestStoreDelete(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	store := NewStore(st.DB)
	ctx := context.Background()

	p, err := store.Create(ctx, "default", "à supprimer", CreateOpts{})
	require.NoError(t, err)

	require.NoError(t, store.Delete(ctx, p.ID))
	_, err = store.GetByID(ctx, p.ID)
	assert.Error(t, err, "get post-delete doit fail")
}

func TestShortName(t *testing.T) {
	// Sous 40 chars : idée renvoyée telle quelle.
	assert.Equal(t, "court", shortName("court"))
	assert.Equal(t, "un mot", shortName("un mot"))
	// Plus de 40 chars : tronque + ellipsis.
	long := "une idée beaucoup plus longue que quarante caractères au total"
	got := shortName(long)
	assert.True(t, len(got) <= len(long))
	assert.Contains(t, got, "…")
}

func TestPlaceholders(t *testing.T) {
	// n<=0 renvoie "''" pour générer un IN valide vide.
	assert.Equal(t, "''", placeholders(0))
	assert.Equal(t, "?", placeholders(1))
	assert.Equal(t, "?,?,?", placeholders(3))
}
