// "Invite a friend": start a game and go to it, for the home page and quick match.

import { goto } from '$app/navigation';
import { createGame } from './game.ts';

export class FriendGame {
	busy = $state(false);
	error = $state('');

	start = async () => {
		this.busy = true;
		this.error = '';
		try {
			await goto(`/game/${await createGame()}`);
		} catch (e) {
			this.error = e instanceof Error ? e.message : String(e);
			this.busy = false;
		}
	};
}
