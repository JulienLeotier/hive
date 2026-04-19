import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { confirmDialog, confirmState, closeConfirm } from './confirm';

describe('confirmDialog store', () => {
	beforeEach(() => {
		// Reset store entre tests. closeConfirm(false) vide tout proprement.
		if (get(confirmState)) closeConfirm(false);
	});

	it('sets state when called', () => {
		const promise = confirmDialog({ message: 'Sûr ?' });
		const state = get(confirmState);
		expect(state).not.toBeNull();
		expect(state?.message).toBe('Sûr ?');
		closeConfirm(true);
		return promise; // consommé
	});

	it('resolves true when confirmed', async () => {
		const promise = confirmDialog({ message: 'X' });
		closeConfirm(true);
		await expect(promise).resolves.toBe(true);
	});

	it('resolves false when cancelled', async () => {
		const promise = confirmDialog({ message: 'X' });
		closeConfirm(false);
		await expect(promise).resolves.toBe(false);
	});

	it('auto-resolves previous when a new dialog opens', async () => {
		const first = confirmDialog({ message: 'first' });
		const second = confirmDialog({ message: 'second' });
		// La première doit être résolue à false automatiquement.
		await expect(first).resolves.toBe(false);
		closeConfirm(true);
		await expect(second).resolves.toBe(true);
	});

	it('respects custom labels and danger flag', () => {
		const promise = confirmDialog({
			message: 'Delete?',
			confirmLabel: 'Oui',
			cancelLabel: 'Non',
			danger: true
		});
		const state = get(confirmState);
		expect(state?.confirmLabel).toBe('Oui');
		expect(state?.cancelLabel).toBe('Non');
		expect(state?.danger).toBe(true);
		closeConfirm(false);
		return promise;
	});
});
