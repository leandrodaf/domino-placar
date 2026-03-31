import html from "eslint-plugin-html";

export default [
  {
    // Ignore files with Go template syntax ({{ }}) inside <script> blocks
    // ESLint cannot parse Go template expressions mixed with JavaScript
    ignores: [
      "templates/game.html",
      "templates/game_lobby.html",
      "templates/lobby.html",
      "templates/ranking.html",
      "templates/round-scores.html",
      "templates/tournament-ranking.html",
      "templates/tournament-waiting.html",
    ],
  },
  {
    files: ["**/*.html"],
    plugins: { html },
  },
  {
    files: ["**/*.js", "**/*.html"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "script",
      globals: {
        window: "readonly",
        document: "readonly",
        console: "readonly",
        navigator: "readonly",
        localStorage: "readonly",
        sessionStorage: "readonly",
        setTimeout: "readonly",
        setInterval: "readonly",
        clearTimeout: "readonly",
        clearInterval: "readonly",
        fetch: "readonly",
        EventSource: "readonly",
        FormData: "readonly",
        URLSearchParams: "readonly",
        alert: "readonly",
        confirm: "readonly",
        prompt: "readonly",
        location: "readonly",
        history: "readonly",
        Image: "readonly",
        FileReader: "readonly",
        Notification: "readonly",
        AbortController: "readonly",
        requestAnimationFrame: "readonly",
      },
    },
    rules: {
      "no-undef": "warn",
      "no-unused-vars": ["warn", { "args": "none", "caughtErrors": "none" }],
      "no-redeclare": "error",
      eqeqeq: "warn",
      "no-eval": "error",
      "no-implied-eval": "error",
    },
  },
];
