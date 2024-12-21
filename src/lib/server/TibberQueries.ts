export const usageQuery = `{
  viewer {
    homes {
      id
      consumption(resolution: HOURLY, last: HOURS_TO_GET) {
        nodes {
          from
          to
          unitPrice
          unitPriceVAT
          consumption
        }
      }
      production(resolution: HOURLY, last: HOURS_TO_GET) {
        nodes {
          from
          to
          unitPrice
          unitPriceVAT
          production
        }
      }
      currentSubscription {
        status
        priceInfo {
          today {
            total
            energy
            tax
            startsAt
          }
          current {
            total
            energy
            tax
            startsAt
          }
        }
      }
    }
  }
}`;
