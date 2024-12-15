<script lang="ts">
	import { onMount } from 'svelte';

	onMount(() => {
		var now = new Date(),
			hourDeg = (now.getHours() / 12) * 360 + (now.getMinutes() / 60) * 30,
			minuteDeg = (now.getMinutes() / 60) * 360 + (now.getSeconds() / 60) * 6,
			secondDeg = (now.getSeconds() / 60) * 360,
			stylesDeg = [
				'@-webkit-keyframes rotate-hour{from{transform:rotate(' +
					hourDeg +
					'deg);}to{transform:rotate(' +
					(hourDeg + 360) +
					'deg);}}',
				'@-webkit-keyframes rotate-minute{from{transform:rotate(' +
					minuteDeg +
					'deg);}to{transform:rotate(' +
					(minuteDeg + 360) +
					'deg);}}',
				'@-webkit-keyframes rotate-second{from{transform:rotate(' +
					secondDeg +
					'deg);}to{transform:rotate(' +
					(secondDeg + 360) +
					'deg);}}',
				'@-moz-keyframes rotate-hour{from{transform:rotate(' +
					hourDeg +
					'deg);}to{transform:rotate(' +
					(hourDeg + 360) +
					'deg);}}',
				'@-moz-keyframes rotate-minute{from{transform:rotate(' +
					minuteDeg +
					'deg);}to{transform:rotate(' +
					(minuteDeg + 360) +
					'deg);}}',
				'@-moz-keyframes rotate-second{from{transform:rotate(' +
					secondDeg +
					'deg);}to{transform:rotate(' +
					(secondDeg + 360) +
					'deg);}}'
			].join('');

		// Create a style element instead of looking for an existing one
		const styleEl = document.createElement('style');
		styleEl.textContent = stylesDeg;
		document.head.appendChild(styleEl);
	});
</script>

<div class="analogClock">
	<div class="clock-base">
		<div class="clock-dial">
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
			<div class="clock-indicator"></div>
		</div>
		<div class="clock-hour"></div>
		<div class="clock-minute"></div>
		<div class="clock-second"></div>
		<div class="clock-center"></div>
	</div>
</div>

<style>
    .analogClock {
        position: relative;

        width: 100%;
        aspect-ratio: 1;
        max-width: 100%;
        margin: auto;
        padding: 2%;
        background-image: linear-gradient(#f7f7f7, #e0e0e0);
        border-radius: 50%;
        box-shadow:
                0 4% 6% rgba(0, 0, 0, 0.15),
                0 1% 1% rgba(0, 0, 0, 0.2);
    }

    .clock-base {
        width: 100%;
        height: 100%;
        background-color: #eee;
        border-radius: 50%;
        box-shadow: 0 0 2% #eee;
    }

    .clock-dial {
        position: absolute;
        z-index: 1;
        top: 4%;
        left: 4%;
        width: 92%;
        height: 92%;
    }

    .clock-dial .clock-indicator {
        position: absolute;
        width: 0.8%;
        height: 1.6%;
        margin: 45.2% 45.6%;
        background-color: #ddd;
    }

    .clock-indicator:nth-child(1) { transform: rotate(30deg) translateY(-45.2%); }
    .clock-indicator:nth-child(2) { transform: rotate(60deg) translateY(-45.2%); }
    .clock-indicator:nth-child(3) {
        transform: rotate(90deg) translateY(-45.2%);
        background-color: #aaa;
    }
    .clock-indicator:nth-child(4) { transform: rotate(120deg) translateY(-45.2%); }
    .clock-indicator:nth-child(5) { transform: rotate(150deg) translateY(-45.2%); }
    .clock-indicator:nth-child(6) {
        transform: rotate(180deg) translateY(-45.2%);
        background-color: #aaa;
    }
    .clock-indicator:nth-child(7) { transform: rotate(210deg) translateY(-45.2%); }
    .clock-indicator:nth-child(8) { transform: rotate(240deg) translateY(-45.2%); }
    .clock-indicator:nth-child(9) {
        transform: rotate(270deg) translateY(-45.2%);
        background-color: #aaa;
    }
    .clock-indicator:nth-child(10) { transform: rotate(300deg) translateY(-45.2%); }
    .clock-indicator:nth-child(11) { transform: rotate(330deg) translateY(-45.2%); }
    .clock-indicator:nth-child(12) {
        transform: rotate(360deg) translateY(-45.2%);
        background-color: #c00;
    }

    .clock-hour {
        position: absolute;
        z-index: 2;
        top: 24%;
        left: 49.2%;
        width: 1.6%;
        height: 26%;
        background-color: #555;
        border-radius: 1%;
        box-shadow: 0 0 2px rgba(0, 0, 0, 0.2);
        transform-origin: 50% 77%;
        transition: 0.5s;
        -webkit-animation: rotate-hour 43200s linear infinite;
        -moz-animation: rotate-hour 43200s linear infinite;
    }

    .clock-minute {
        position: absolute;
        z-index: 3;
        top: 10%;
        left: 49.2%;
        width: 1.6%;
        height: 40%;
        background-color: #555;
        border-radius: 1%;
        box-shadow: 0 0 2px rgba(0, 0, 0, 0.2);
        transform-origin: 50% 82%;
        transition: 0.5s;
        -webkit-animation: rotate-minute 3600s linear infinite;
        -moz-animation: rotate-minute 3600s linear infinite;
    }

    .clock-second {
        position: absolute;
        z-index: 4;
        top: 3%;
        left: 49.6%;
        width: 0.8%;
        height: 47%;
        background-color: #a00;
        box-shadow: 0 0 2px rgba(0, 0, 0, 0.2);
        transform-origin: 50% 85%;
        transition: 0.5s;
        -webkit-animation: rotate-second 60s linear infinite;
        -moz-animation: rotate-second 60s linear infinite;
    }

    .clock-second:after {
        content: '';
        display: block;
        position: absolute;
        left: -150%;
        bottom: 11%;
        width: 300%;
        aspect-ratio: 1;
        background-color: #a00;
        border: solid 1px #a00;
        border-radius: 50%;
        box-shadow: 0 0 3px rgba(0, 0, 0, 0.2);
    }

    .clock-center {
        position: absolute;
        z-index: 1;
        width: 60%;
        height: 60%;
        top: 20%;
        left: 20%;
        background-image: linear-gradient(#e3e3e3, #f7f7f7);
        border-radius: 50%;
        box-shadow:
                inset 0 -1px 0 #fafafa,
                inset 0 1px 0 #e3e3e3;
    }

    .clock-center:after {
        content: '';
        display: block;
        width: 13.33%;
        height: 13.33%;
        margin: 43.33%;
        background-color: #ddd;
        border-radius: 50%;
    }
</style>
