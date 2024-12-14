/**
 * This file is the client side of the dashboard. It is not run on the server, but served as is to the client.
 */

let eventsHash = "";
let version = "";

let ws = null;

let wsRetryCount = 0;

const wsId =
  Math.random().toString(36).substring(2, 15) +
  Math.random().toString(36).substring(2, 15);

let lastUpdatedPower = new Date();

// Reload the page every fifteen seconds
window.addEventListener("DOMContentLoaded", function () {
  setReloadClient(1);
  console.log("Client loaded, will reload at the start of the next hour");
});

/**
 * Connect to the WebSocket and handle reconnections and messages.
 */
async function connectWebSocket() {
  console.log("Connecting to WebSocket");
  ws = new WebSocket(
    "ws://" + window.location.hostname + ":" + window.location.port,
  );
  ws.onopen = function () {
    // Web Socket is connected, identify ourselves
    ws.send(
      JSON.stringify({
        message: `Hello, from ${wsId}!`,
        type: "identify",
        id: wsId,
      }),
    );
  };

  ws.onclose = function () {
    document.getElementById("now_time").innerHTML = `:(`;
    document.getElementById("now_date").innerHTML = ``;
  };

  ws.onmessage = function (evt) {
    try {
      const data = JSON.parse(evt.data);
      if (data.version && version == "") {
        // Setting the version
        console.log("Setting version to", data.version);
        version = data.version;
      } else if (data.version && data.version !== "") {
        // Checking if the version is the same, reload if not
        if (version !== data.version) {
          window.location.reload();
        }
      }
      if (data.time) {
        updateTimeInfo(data.time);
      }
      if (data.homey) {
        updateHomeyInfo(data.homey);
      }
      if (data.entur) {
        updateEnturInfo(data.entur);
      }
      if (data.powerPrice) {
        const currentPowerPrice = data.powerPrice;
        // Round to two decimals
        const roundedPrice = Math.round(currentPowerPrice * 100) / 100;
        document.getElementById("currentPriceHome").innerHTML =
          `${roundedPrice} kr/kWh`;
      }
      if (data.eventsHash) {
        if (eventsHash === "") {
          eventsHash = data.eventsHash;
        } else if (eventsHash !== "" && eventsHash !== data.eventsHash) {
          console.log("Events hash updated");
          window.location.reload();
        }
      }
    } catch (e) {
      console.log(e);
    }
  };
}

// Run a loop to check if the WebSocket is open
function checkWebSocket() {
  if (ws === null) {
    connectWebSocket();
  }
  else if (ws.readyState !== 1) {
     // Reload window if we have retried too many times
     console.log("WebSocket not open, trying to connect. Tries:", wsRetryCount);
     // 15 minutes as we wait for 10 seconds per try
      if (wsRetryCount > (15 * 6)) {
        console.log("Too many retries, giving up");
        window.location.reload();
      }
      wsRetryCount++;
    connectWebSocket();
  } else {
    wsRetryCount = 0;
    console.log("WebSocket is open");
  }
  setTimeout(checkWebSocket, 10000);
}

checkWebSocket();

/**
 * Update the UX with Homey data.
 * @param {HomeyData} homey
 */
function updateHomeyInfo(homey) {
  // Show temperature
  if (homey.tempOut) {
    const t = Number(homey.tempOut).toFixed(1);
    if (t === "-0") {
      document.getElementById("current_temperature").innerHTML = `0&deg;`;
    } else {
      document.getElementById("current_temperature").innerHTML = `${t}&deg;`;
    }
  } else {
    document.getElementById("current_temperature").innerHTML = `?`;
  }

  // Show pressure
  if (homey.pressure) {
    const p = Number(homey.pressure).toFixed(0);
    document.getElementById("current_pressure").innerHTML = `${p} hPa`;
  }

  // Show humidity
  if (homey.humOut) {
    const p = Number(homey.humOut).toFixed(0);
    document.getElementById("current_humidity").innerHTML = `${p} % hum`;
  }

  if (homey.power) {
    setLastUpdatedPowerTime();
    const p = Math.round(Number(homey.power) / 100) / 10;
    document.getElementById("current_power").innerHTML = `${p} kW`;
  }
  if (homey.powerUsedToday) {
    document.getElementById("powerUsageTodayHome").innerHTML = Math.round(
      Number(homey.powerUsedToday),
    ).toString();
  }
  if (homey.powerUsedTodayCabin) {
    document.getElementById("powerUsageTodayCabin").innerHTML = Math.round(
      Number(homey.powerUsedTodayCabin),
    ).toString();
  }
  if (homey.powerCabin !== undefined) {
    const p = Math.round(Number(homey.powerCabin) / 100) / 10;
    document.getElementById("currentPowerCabin").innerHTML = `${p} kW`;
  }
  if (homey.coolerRoomTemp !== null) {
    const coolerTemp = homey.coolerRoomTemp;
    document.getElementById("coolerTemp").innerHTML = coolerTemp.toString();
  }
  if (homey.coolerRoomHumidity !== null) {
    const coolerHum = homey.coolerRoomHumidity;
    document.getElementById("coolerHumidity").innerHTML = coolerHum.toString();
  }

  // Remove power data if not updated lately
  checkLastUpdatedPowerTime();
}

/**
 * Set the last updated power time to the current time.
 */
function setLastUpdatedPowerTime() {
  lastUpdatedPower = new Date();
}

/**
 * Check the last updated power time and update the UI if too much time has passed.
 */
function checkLastUpdatedPowerTime() {
  const now = new Date();
  const diff = now.getTime() - lastUpdatedPower.getTime();
  const diffMinutes = Math.round(diff / 1000 / 60);

  if (diffMinutes > 1) {
    document.getElementById("current_power").innerHTML = `?`;
    document.getElementById("current_price").innerHTML = `?`;
    document.getElementById("powerUsageTodayHome").innerHTML = `?`;
    document.getElementById("powerCostTodayHome").innerHTML = `?`;
  }
}

/**
 * Update the time information in the UI.
 * @param {TimeData} timeData
 */
function updateTimeInfo(timeData) {
  // Update all the interfaces at once
  document.getElementById("now_time").innerHTML = timeData.time;
  document.getElementById("now_date").innerHTML = timeData.date;
  document.getElementById("now_week").innerHTML = `Uke ${timeData.week}`;
}

/**
 * Update the train information in the UI.
 * @param {Train[]} entur
 */
function updateEnturInfo(entur) {
  let enturHtml = "<strong>Neste baner</strong>";

  for (let i = 0; i < Math.min(entur.length, 4); i++) {
    enturHtml += `<span class="entur_item">${entur[i].time.substring(
      11,
      16,
    )}</span>`;
  }

  document.getElementById("bane").innerHTML = enturHtml;
}

/**
 * Reload the page at the start of the next hour.
 * @param {number} inHours
 */
function setReloadClient(inHours) {
  const now = new Date();
  const startOfNextHour = new Date();
  startOfNextHour.setUTCHours(now.getUTCHours() + inHours, 0, 1, 0);
  const diff = startOfNextHour.getTime() - now.getTime();
  setTimeout(() => window.location.reload(), diff);
}
