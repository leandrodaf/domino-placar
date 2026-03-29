import html from "eslint-plugin-html";

export default [
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
        location: "readonly",
        history: "readonly",
        Image: "readonly",
        FileReader: "readonly",
        Notification: "readonly",
        AbortController: "readonly",
      },
    },
    rules: {
      "no-undef": "warn",
      "no-unused-vars": "warn",
      "no-redeclare": "error",
      eqeqeq: "warn",
      "no-eval": "error",
      "no-implied-eval": "error",
    },
  },
];
