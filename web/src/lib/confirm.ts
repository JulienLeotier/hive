// confirm() store : une modal globale qui remplace window.confirm. Les
// callers appellent `await confirmDialog({ title, message })` et
// récupèrent un boolean. Le composant ConfirmHost (monté une fois dans
// +layout.svelte) écoute le store et affiche la modal.
//
// Pourquoi pas un simple composant inline par page : on en a 7+
// usages répartis dans 3 fichiers. Un store + host évite de recopier
// le state (open, onConfirm, onCancel) dans chaque page et garantit
// un comportement uniforme (focus trap, Esc, backdrop).

import { writable } from 'svelte/store';

export type ConfirmOptions = {
	title?: string;
	message: string;
	confirmLabel?: string;
	cancelLabel?: string;
	danger?: boolean; // colorie le bouton primaire en rouge
};

type DialogState = ConfirmOptions & {
	id: number;
	resolve: (value: boolean) => void;
};

export const confirmState = writable<DialogState | null>(null);

let nextID = 1;

// confirmDialog ouvre une modal et résout avec true (confirmé) ou
// false (annulé / Esc / click backdrop). Si une modal est déjà ouverte
// elle est résolue à false avant d'en ouvrir une nouvelle — pas de
// pile, on reste simple.
export function confirmDialog(opts: ConfirmOptions): Promise<boolean> {
	return new Promise((resolve) => {
		confirmState.update((prev) => {
			if (prev) prev.resolve(false);
			return { ...opts, id: nextID++, resolve };
		});
	});
}

// closeConfirm résout la modal courante avec la valeur donnée et la
// ferme. Appelé par le composant ConfirmHost sur Confirm / Cancel.
export function closeConfirm(value: boolean): void {
	confirmState.update((prev) => {
		if (prev) prev.resolve(value);
		return null;
	});
}
