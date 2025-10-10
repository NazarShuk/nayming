<script lang="ts">
	import { onMount } from 'svelte';

	let serverAddress = $state('ws://localhost:8080');
	let messages: string[] = $state([]);

	let ws: WebSocket | null = $state(null);
	let connection: RTCPeerConnection | null = $state(null);
	let mouseChannel: RTCDataChannel | null = $state(null);

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
				const el = document.createElement(event.track.kind) as HTMLMediaElement;
				el.srcObject = event.streams[0];
				el.autoplay = true;
				el.controls = true;
				el.style.width = '1920px';
				el.style.height = '1080px';
				el.onclick = (event) => {
					event.preventDefault();
					mouseChannel?.send(
						JSON.stringify({
							type: 'click',
							x: event.clientX,
							y: event.clientY,
							button: event.button
						})
					);
				};
				el.onmousemove = function (event) {
					mouseChannel?.send(
						JSON.stringify({
							type: 'move',
							x: event.clientX,
							y: event.clientY
						})
					);
				};

				document.body.appendChild(el);
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

<input bind:value={serverAddress} class="border-1 border-black" placeholder="server address" />
<button onclick={connect}>connect</button>

{#each messages as msg, i (i)}
	<p>{msg}</p>
{/each}
