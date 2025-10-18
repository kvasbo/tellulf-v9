<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';

	type AnimationType = 'flash' | 'rain' | 'matrix' | 'explode' | 'wave' | 'spiral' | 'glitch' | 'bounce' | 'typewriter' | 'fireworks';

	let visible = false;
	let activeAnimation: AnimationType = 'flash';
	let checkInterval: number;

	const animations: AnimationType[] = ['flash', 'rain', 'matrix', 'explode', 'wave', 'spiral', 'glitch', 'bounce', 'typewriter', 'fireworks'];
	const haaaText = 'HAAAAAAAAAAAAAAAAAAAAAAAA';

	const isActiveDate = (date: Date): boolean => {
		const year = date.getFullYear();
		const month = date.getMonth();
		const day = date.getDate();

		return (
			year === 2025 &&
			month === 9 &&
			(day === 30 || day === 31) ||
			(year === 2025 && month === 10 && day === 1)
		);
	};

	const isActiveTime = (date: Date): boolean => {
		const norwegianTime = new Date(date.toLocaleString('en-US', { timeZone: 'Europe/Oslo' }));
		const hour = norwegianTime.getHours();
		return hour >= 20 && hour < 23;
	};

	const hasHaaParam = (): boolean => {
		if (!browser) return false;
		return new URLSearchParams(window.location.search).has('haa');
	};

	const triggerAnimation = () => {
		const now = new Date();

		if (hasHaaParam() || (isActiveDate(now) && isActiveTime(now))) {
			activeAnimation = animations[Math.floor(Math.random() * animations.length)];
			visible = true;

			setTimeout(() => {
				visible = false;
			}, 60000);
		}
	};

	onMount(() => {
		if (browser) {
			checkInterval = setInterval(triggerAnimation, 12 * 60 * 1000) as unknown as number;
			triggerAnimation();
		}
	});

	onDestroy(() => {
		if (checkInterval) {
			clearInterval(checkInterval);
		}
	});

	const generateRainItems = () => {
		return Array.from({ length: 50 }, (_, i) => ({
			text: haaaText,
			left: Math.random() * 100,
			delay: Math.random() * 2,
			duration: 2 + Math.random() * 2
		}));
	};

	const generateMatrixItems = () => {
		return Array.from({ length: 30 }, (_, i) => ({
			text: haaaText,
			left: (i * 3.33) % 100,
			delay: Math.random() * 3,
			duration: 3 + Math.random() * 2
		}));
	};

	const generateExplodeItems = () => {
		return Array.from({ length: 40 }, (_, i) => {
			const angle = (i * 360) / 40;
			return {
				text: haaaText,
				angle,
				delay: Math.random() * 0.5
			};
		});
	};

	const generateWaveItems = () => {
		return Array.from({ length: 20 }, (_, i) => ({
			text: haaaText,
			index: i,
			delay: i * 0.1
		}));
	};

	const generateSpiralItems = () => {
		return Array.from({ length: 50 }, (_, i) => ({
			text: haaaText,
			index: i,
			angle: i * 20,
			delay: i * 0.05
		}));
	};

	const generateBounceItems = () => {
		return Array.from({ length: 15 }, (_, i) => ({
			text: haaaText,
			left: (i * 6.66) % 100,
			delay: i * 0.2
		}));
	};

	const generateFireworksItems = () => {
		return Array.from({ length: 30 }, (_, i) => ({
			text: haaaText,
			x: Math.random() * 100,
			y: Math.random() * 100,
			delay: Math.random() * 2
		}));
	};

	$: rainItems = generateRainItems();
	$: matrixItems = generateMatrixItems();
	$: explodeItems = generateExplodeItems();
	$: waveItems = generateWaveItems();
	$: spiralItems = generateSpiralItems();
	$: bounceItems = generateBounceItems();
	$: fireworksItems = generateFireworksItems();
</script>

{#if visible}
	<div class="haaa-overlay" class:visible>
		{#if activeAnimation === 'flash'}
			<div class="flash-container">
				{#each Array(10) as _, i}
					<div class="flash-text" style="animation-delay: {i * 0.2}s;">{haaaText}</div>
				{/each}
			</div>
		{:else if activeAnimation === 'rain'}
			<div class="rain-container">
				{#each rainItems as item}
					<div
						class="rain-text"
						style="left: {item.left}%; animation-delay: {item.delay}s; animation-duration: {item.duration}s;"
					>
						{item.text}
					</div>
				{/each}
			</div>
		{:else if activeAnimation === 'matrix'}
			<div class="matrix-container">
				{#each matrixItems as item}
					<div
						class="matrix-text"
						style="left: {item.left}%; animation-delay: {item.delay}s; animation-duration: {item.duration}s;"
					>
						{item.text}
					</div>
				{/each}
			</div>
		{:else if activeAnimation === 'explode'}
			<div class="explode-container">
				{#each explodeItems as item}
					<div
						class="explode-text"
						style="--angle: {item.angle}deg; animation-delay: {item.delay}s;"
					>
						{item.text}
					</div>
				{/each}
			</div>
		{:else if activeAnimation === 'wave'}
			<div class="wave-container">
				{#each waveItems as item}
					<div class="wave-text" style="animation-delay: {item.delay}s; top: {item.index * 5}%;">
						{item.text.repeat(5)}
					</div>
				{/each}
			</div>
		{:else if activeAnimation === 'spiral'}
			<div class="spiral-container">
				{#each spiralItems as item}
					<div
						class="spiral-text"
						style="--angle: {item.angle}deg; --radius: {item.index * 2}vh; animation-delay: {item.delay}s;"
					>
						{item.text}
					</div>
				{/each}
			</div>
		{:else if activeAnimation === 'glitch'}
			<div class="glitch-container">
				<div class="glitch-text" data-text={haaaText}>{haaaText}</div>
			</div>
		{:else if activeAnimation === 'bounce'}
			<div class="bounce-container">
				{#each bounceItems as item}
					<div class="bounce-text" style="left: {item.left}%; animation-delay: {item.delay}s;">
						{item.text}
					</div>
				{/each}
			</div>
		{:else if activeAnimation === 'typewriter'}
			<div class="typewriter-container">
				<div class="typewriter-text">{haaaText.repeat(20)}</div>
			</div>
		{:else if activeAnimation === 'fireworks'}
			<div class="fireworks-container">
				{#each fireworksItems as item}
					<div
						class="fireworks-text"
						style="left: {item.x}%; top: {item.y}%; animation-delay: {item.delay}s;"
					>
						{item.text}
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	.haaa-overlay {
		position: fixed;
		top: 0;
		left: 0;
		width: 100vw;
		height: 100vh;
		z-index: 9999;
		pointer-events: none;
		overflow: hidden;
		opacity: 0;
		transition: opacity 0.3s;
	}

	.haaa-overlay.visible {
		opacity: 1;
	}

	.flash-container {
		width: 100%;
		height: 100%;
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;
		background: rgba(0, 0, 0, 0.5);
	}

	.flash-text {
		font-size: 6rem;
		font-weight: bold;
		color: #fff;
		animation: flash 0.5s infinite;
		text-shadow: 0 0 20px rgba(255, 255, 255, 0.8);
		word-break: break-all;
		text-align: center;
		padding: 0 2rem;
	}

	@keyframes flash {
		0%,
		100% {
			opacity: 0;
		}
		50% {
			opacity: 1;
		}
	}

	.rain-container {
		width: 100%;
		height: 100%;
		position: relative;
	}

	.rain-text {
		position: absolute;
		top: -10%;
		font-size: 2rem;
		font-weight: bold;
		color: #4a9eff;
		animation: rain-fall 3s linear infinite;
		white-space: nowrap;
	}

	@keyframes rain-fall {
		0% {
			top: -10%;
			opacity: 1;
		}
		100% {
			top: 110%;
			opacity: 0.3;
		}
	}

	.matrix-container {
		width: 100%;
		height: 100%;
		position: relative;
		background: rgba(0, 0, 0, 0.8);
	}

	.matrix-text {
		position: absolute;
		top: -10%;
		font-size: 1.5rem;
		font-weight: bold;
		color: #00ff00;
		animation: matrix-fall 4s linear infinite;
		font-family: monospace;
		text-shadow: 0 0 10px #00ff00;
	}

	@keyframes matrix-fall {
		0% {
			top: -10%;
			opacity: 1;
		}
		100% {
			top: 110%;
			opacity: 0;
		}
	}

	.explode-container {
		width: 100%;
		height: 100%;
		position: relative;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.explode-text {
		position: absolute;
		font-size: 3rem;
		font-weight: bold;
		color: #ff4a4a;
		animation: explode 2s ease-out forwards;
		transform-origin: center;
	}

	@keyframes explode {
		0% {
			transform: translate(0, 0) scale(0.1) rotate(0deg);
			opacity: 1;
		}
		100% {
			transform: translate(
					calc(cos(var(--angle)) * 150vh),
					calc(sin(var(--angle)) * 150vh)
				)
				scale(1) rotate(720deg);
			opacity: 0;
		}
	}

	.wave-container {
		width: 100%;
		height: 100%;
		position: relative;
		overflow: hidden;
	}

	.wave-text {
		position: absolute;
		left: -100%;
		width: 200%;
		font-size: 3rem;
		font-weight: bold;
		color: #ff00ff;
		animation: wave-slide 3s ease-in-out infinite;
		text-shadow: 0 0 15px rgba(255, 0, 255, 0.6);
		overflow: hidden;
		text-overflow: clip;
	}

	@keyframes wave-slide {
		0% {
			left: -100%;
		}
		50% {
			left: 100%;
		}
		100% {
			left: -100%;
		}
	}

	.spiral-container {
		width: 100%;
		height: 100%;
		position: relative;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.spiral-text {
		position: absolute;
		font-size: 2.5rem;
		font-weight: bold;
		color: #ffa500;
		animation: spiral-spin 4s linear infinite;
	}

	@keyframes spiral-spin {
		0% {
			transform: translate(0, 0) rotate(0deg) scale(0);
			opacity: 0;
		}
		50% {
			opacity: 1;
		}
		100% {
			transform: translate(
					calc(cos(var(--angle)) * var(--radius)),
					calc(sin(var(--angle)) * var(--radius))
				)
				rotate(calc(var(--angle) * 2)) scale(1.5);
			opacity: 0;
		}
	}

	.glitch-container {
		width: 100%;
		height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		background: rgba(0, 0, 0, 0.9);
	}

	.glitch-text {
		font-size: 6rem;
		font-weight: bold;
		color: #fff;
		position: relative;
		animation: glitch-main 0.5s infinite;
		word-break: break-all;
		text-align: center;
		padding: 0 2rem;
		max-width: 90vw;
	}

	.glitch-text::before,
	.glitch-text::after {
		content: attr(data-text);
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
	}

	.glitch-text::before {
		color: #ff00ff;
		animation: glitch-before 0.3s infinite;
		clip-path: polygon(0 0, 100% 0, 100% 45%, 0 45%);
	}

	.glitch-text::after {
		color: #00ffff;
		animation: glitch-after 0.3s infinite;
		clip-path: polygon(0 60%, 100% 60%, 100% 100%, 0 100%);
	}

	@keyframes glitch-main {
		0% {
			transform: translate(0);
		}
		20% {
			transform: translate(-2px, 2px);
		}
		40% {
			transform: translate(-2px, -2px);
		}
		60% {
			transform: translate(2px, 2px);
		}
		80% {
			transform: translate(2px, -2px);
		}
		100% {
			transform: translate(0);
		}
	}

	@keyframes glitch-before {
		0% {
			transform: translate(0);
		}
		20% {
			transform: translate(2px, -2px);
		}
		40% {
			transform: translate(-2px, 2px);
		}
		60% {
			transform: translate(2px, 2px);
		}
		80% {
			transform: translate(-2px, -2px);
		}
		100% {
			transform: translate(0);
		}
	}

	@keyframes glitch-after {
		0% {
			transform: translate(0);
		}
		20% {
			transform: translate(-2px, 2px);
		}
		40% {
			transform: translate(2px, -2px);
		}
		60% {
			transform: translate(-2px, -2px);
		}
		80% {
			transform: translate(2px, 2px);
		}
		100% {
			transform: translate(0);
		}
	}

	.bounce-container {
		width: 100%;
		height: 100%;
		position: relative;
	}

	.bounce-text {
		position: absolute;
		top: 0;
		font-size: 3rem;
		font-weight: bold;
		color: #ff1493;
		animation: bounce-drop 2s ease-in-out infinite;
	}

	@keyframes bounce-drop {
		0%,
		100% {
			top: -10%;
		}
		25% {
			top: 70%;
		}
		35% {
			top: 50%;
		}
		45% {
			top: 70%;
		}
		55% {
			top: 60%;
		}
		65% {
			top: 70%;
		}
		75% {
			top: 67%;
		}
		85% {
			top: 70%;
		}
	}

	.typewriter-container {
		width: 100%;
		height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		background: rgba(0, 0, 0, 0.8);
		overflow: hidden;
	}

	.typewriter-text {
		font-size: 3rem;
		font-weight: bold;
		color: #00ff00;
		font-family: monospace;
		white-space: nowrap;
		overflow: hidden;
		border-right: 3px solid #00ff00;
		animation: typewriter 4s steps(100) infinite, blink 0.5s step-end infinite;
		max-width: 95vw;
		padding: 0 1rem;
	}

	@keyframes typewriter {
		0% {
			width: 0;
		}
		50% {
			width: 100%;
		}
		100% {
			width: 0;
		}
	}

	@keyframes blink {
		50% {
			border-color: transparent;
		}
	}

	.fireworks-container {
		width: 100%;
		height: 100%;
		position: relative;
		background: rgba(0, 0, 20, 0.9);
	}

	.fireworks-text {
		position: absolute;
		font-size: 2rem;
		font-weight: bold;
		animation: fireworks-burst 2s ease-out infinite;
	}

	@keyframes fireworks-burst {
		0% {
			transform: scale(0) rotate(0deg);
			opacity: 1;
			color: hsl(0, 100%, 50%);
		}
		25% {
			color: hsl(90, 100%, 50%);
		}
		50% {
			transform: scale(1.5) rotate(180deg);
			color: hsl(180, 100%, 50%);
		}
		75% {
			color: hsl(270, 100%, 50%);
		}
		100% {
			transform: scale(0) rotate(360deg);
			opacity: 0;
			color: hsl(360, 100%, 50%);
		}
	}
</style>
