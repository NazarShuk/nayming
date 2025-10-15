<script lang="ts">
	import { onMount } from 'svelte';

	let serverAddress = $state('ws://localhost:8080');

	let ws: WebSocket | null = $state(null);
	let connection: RTCPeerConnection | null = $state(null);
	let mouseChannel: RTCDataChannel | null = $state(null);

	let videoElement: HTMLVideoElement | undefined = $state();

	function connect() {
		ws = new WebSocket(`${serverAddress}/ws`);
		ws.onopen = async () => {
			console.log('WebSocket connected');

			// Create peer connection
			connection = new RTCPeerConnection({
				iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
			});

			connection.ontrack = function (event) {
				console.log(event.track.kind);
				if (videoElement) {
					videoElement.srcObject = event.streams[0];
					videoElement.play();
				}
			};

			connection.addTransceiver('video', { direction: 'sendrecv' });

			connection.createDataChannel('alive');
			mouseChannel = connection.createDataChannel('mouse');

			// Send ICE candidates to server
			connection.onicecandidate = (event) => {
				if (event.candidate) {
					ws?.send(
						JSON.stringify({
							type: 'candidate',
							candidate: JSON.stringify(event.candidate.toJSON())
						})
					);
				}
			};

			// Handle connection state
			connection.onconnectionstatechange = () => {
				console.log('Status: ' + connection?.connectionState);
			};

			// Create and send offer
			const offer = await connection.createOffer();
			await connection.setLocalDescription(offer);
			ws?.send(
				JSON.stringify({
					type: 'offer',
					sdp: offer.sdp
				})
			);
		};

		ws.onmessage = async (event) => {
			const msg = JSON.parse(event.data);

			if (msg.type === 'answer') {
				await connection?.setRemoteDescription({
					type: 'answer',
					sdp: msg.sdp
				});
			} else if (msg.type === 'candidate') {
				const candidate = JSON.parse(msg.candidate);
				await connection?.addIceCandidate(candidate);
			}
		};
	}

	onMount(() => {
		return () => {
			if (ws) {
				ws.close();
			}
			if (connection) {
				connection.close();
			}
		};
	});
</script>

<div class="flex h-screen w-full flex-col items-center justify-center bg-neutral-950 text-white">
	{#if connection}
		<video class="h-full w-full" bind:this={videoElement} />
	{:else}
		<div class="h-1/2 w-1/2 rounded bg-neutral-900 p-2.5">
			<h1 class="mb-5 text-xl font-bold">Connect</h1>
			<form
				class="flex flex-row justify-between gap-5"
				onsubmit={(e) => {
					e.preventDefault();
					connect();
				}}
			>
				<input
					bind:value={serverAddress}
					class="w-full rounded bg-neutral-800 p-1"
					placeholder="server address"
				/>
				<button class="rounded bg-neutral-800 p-1" type="submit">Connect</button>
			</form>
		</div>
	{/if}
</div>
