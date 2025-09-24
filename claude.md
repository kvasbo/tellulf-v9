# Tellulf

## What this is?

This is a status display used in a home, to show weather, calendars, subway departures and power usage (via APIs).

### APIs

Tibber: Power usage and cost. Used via the https://github.com/bisand/tibber-api package.

## Current tasks

On October 1st 2025, "Norgespris" will go live in norway. This means that for the "Home" location in Tibber will pay 0.50 kroner per kwh up to 5000kwh/month and the market (spot) price thereafter, while the "cabin" location will have a cap at 1000 kwh before it pays the spot price after paying .50 kroner up to that point. That means we need an all new cost calculation function.

We have access to both live usage and historical per hour/day, but not all is implemented in the current version (src/lib/server/Tibber.ts).

What we need to do is to calculate the power usage up until this second, and then calculate the cost based on the price of 50 øre kwh for all power usage up to 5000/1000kwh and then use the spot price thereafter.
