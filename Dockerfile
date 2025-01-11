# Use Node.js LTS version as the base image
FROM node:22

# Set working directory
WORKDIR /app

# Install yarn globally
RUN corepack enable && corepack prepare pnpm@9.15.3 --activate

# Copy package files
COPY package.json pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy the rest of the application
COPY . .

# Build the application
RUN pnpm build

# Expose the port the app runs on
EXPOSE 3000

# Set environment variables
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000

# Start the application
CMD ["pnpm", "start"]