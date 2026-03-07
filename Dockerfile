FROM node:22

WORKDIR /app

COPY package.json package-lock.json ./

RUN npm ci

COPY . .

EXPOSE 3000

ENV NODE_ENV=production
ENV HOST=0.0.0.0

CMD ["npm", "start"]
