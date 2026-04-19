import { describe, it, expect } from 'vitest';
import { fmtRelative } from './format';

describe('fmtRelative', () => {
	it('renders "à l\'instant" pour il y a quelques secondes', () => {
		const now = new Date();
		const out = fmtRelative(now.toISOString());
		expect(out).toMatch(/instant|secondes?|\u00e0 l/);
	});

	it('renders minutes', () => {
		const d = new Date(Date.now() - 5 * 60 * 1000);
		expect(fmtRelative(d.toISOString())).toContain('min');
	});

	it('renders hours', () => {
		const d = new Date(Date.now() - 3 * 3600 * 1000);
		expect(fmtRelative(d.toISOString())).toContain('h');
	});

	it('renders days', () => {
		const d = new Date(Date.now() - 3 * 86400 * 1000);
		expect(fmtRelative(d.toISOString())).toMatch(/j|jour/);
	});

	it('accepte un timestamp undefined en renvoyant quelque chose de safe', () => {
		// Vérifie pas de throw — la signature tolère les inputs vides.
		expect(() => fmtRelative('')).not.toThrow();
	});
});
