import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import ConfirmHost from './ConfirmHost.svelte';
import { confirmDialog, closeConfirm, confirmState } from './confirm';
import { get } from 'svelte/store';

describe('ConfirmHost', () => {
	afterEach(() => {
		if (get(confirmState)) closeConfirm(false);
		cleanup();
	});

	it('ne rend rien tant que confirmDialog n\'est pas appelé', () => {
		render(ConfirmHost);
		// Le dialog ne doit pas être dans le DOM.
		expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
	});

	it('affiche message + deux boutons quand ouvert', async () => {
		render(ConfirmHost);
		const promise = confirmDialog({
			title: 'Confirmer ?',
			message: 'Ce geste va supprimer X',
			confirmLabel: 'Oui',
			cancelLabel: 'Non'
		});
		// On attend que Svelte rerende.
		await Promise.resolve();
		expect(screen.getByText('Confirmer ?')).toBeInTheDocument();
		expect(screen.getByText('Ce geste va supprimer X')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Oui' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Non' })).toBeInTheDocument();
		closeConfirm(false);
		await promise;
	});

	it('clic confirm → resolve true', async () => {
		render(ConfirmHost);
		const promise = confirmDialog({ message: '?' });
		await Promise.resolve();
		const confirmBtn = screen.getByRole('button', { name: /confirmer/i });
		await fireEvent.click(confirmBtn);
		await expect(promise).resolves.toBe(true);
	});

	it('clic cancel → resolve false', async () => {
		render(ConfirmHost);
		const promise = confirmDialog({ message: '?' });
		await Promise.resolve();
		const cancelBtn = screen.getByRole('button', { name: /annuler/i });
		await fireEvent.click(cancelBtn);
		await expect(promise).resolves.toBe(false);
	});
});
