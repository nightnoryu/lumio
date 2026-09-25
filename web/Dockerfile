FROM node:26-alpine

WORKDIR /app

RUN npm install -g pnpm@12

COPY package.json .
COPY pnpm-lock.yaml .

RUN pnpm install --frozen-lockfile

COPY src .
COPY index.html .
COPY tsconfig.json .
COPY vite.config.ts .

EXPOSE 3000
CMD [ "pnpm", "run", "dev" ]
