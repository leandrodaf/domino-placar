package net.dominoplacar.app;

import android.Manifest;
import android.annotation.SuppressLint;
import android.content.ActivityNotFoundException;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.ConnectivityManager;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.NetworkRequest;
import android.net.Uri;
import android.os.Bundle;
import android.provider.MediaStore;
import android.util.Log;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.PermissionRequest;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.ProgressBar;
import android.widget.Toast;

import androidx.activity.OnBackPressedCallback;
import androidx.activity.result.ActivityResultLauncher;
import androidx.activity.result.contract.ActivityResultContracts;
import androidx.annotation.NonNull;
import androidx.appcompat.app.AppCompatActivity;
import androidx.core.content.ContextCompat;
import androidx.core.content.FileProvider;
import androidx.core.splashscreen.SplashScreen;
import androidx.core.view.WindowCompat;
import androidx.core.view.WindowInsetsCompat;
import androidx.core.view.WindowInsetsControllerCompat;

import java.io.File;
import java.io.IOException;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.HashMap;
import java.util.Locale;
import java.util.Map;

/**
 * MainActivity — WebView wrapper de produção para dominoplacar.net
 *
 * Features:
 * - WebView fullscreen imersivo (sem barras do sistema)
 * - Accept-Language automático (i18n via idioma do dispositivo)
 * - Wake Lock (tela sempre ligada durante partidas)
 * - Cookies + DOM Storage (sessão persistente)
 * - Upload de arquivos + câmera (com FileProvider)
 * - Página offline branded quando sem internet
 * - Barra de progresso de carregamento
 * - Monitoramento de conectividade com reconexão automática
 * - Navegação confinada ao domínio (links externos → navegador)
 * - Deep links para dominoplacar.net
 * - Splash screen (Android 12+ compat)
 * - Back navigation inteligente
 * - Injeção de classe CSS para regras mobile-only
 */
public class MainActivity extends AppCompatActivity {

    private static final String TAG = "DominoPlacar";
    private static final String BASE_URL = "https://dominoplacar.net";
    private static final String ALLOWED_HOST = "dominoplacar.net";

    // Estado visível para o DominoPushService (foreground check)
    static boolean isInForeground = false;
    static String currentMatchId = null;

    // WebView e UI
    private WebView webView;
    private ProgressBar progressBar;
    private View offlineContainer;

    // Upload de arquivo/câmera
    private ValueCallback<Uri[]> fileUploadCallback;
    private Uri cameraPhotoUri;

    // Rede
    private ConnectivityManager connectivityManager;
    private ConnectivityManager.NetworkCallback networkCallback;
    private boolean isConnected = true;
    private boolean isShowingOffline = false;

    // Splash
    private boolean isWebViewReady = false;

    // ── Activity Result Launchers ──────────────────────────────

    private final ActivityResultLauncher<Intent> fileChooserLauncher =
        registerForActivityResult(
            new ActivityResultContracts.StartActivityForResult(),
            result -> {
                if (fileUploadCallback == null) return;

                Uri[] results = null;
                if (result.getResultCode() == RESULT_OK) {
                    if (result.getData() != null && result.getData().getDataString() != null) {
                        results = new Uri[]{Uri.parse(result.getData().getDataString())};
                    } else if (cameraPhotoUri != null) {
                        results = new Uri[]{cameraPhotoUri};
                    }
                }

                fileUploadCallback.onReceiveValue(results);
                fileUploadCallback = null;
                cameraPhotoUri = null;
            }
        );

    private final ActivityResultLauncher<String> cameraPermissionLauncher =
        registerForActivityResult(
            new ActivityResultContracts.RequestPermission(),
            isGranted -> Log.d(TAG, "Camera permission " + (isGranted ? "granted" : "denied"))
        );

    private final ActivityResultLauncher<String> notificationPermissionLauncher =
        registerForActivityResult(
            new ActivityResultContracts.RequestPermission(),
            isGranted -> Log.d(TAG, "Notification permission " + (isGranted ? "granted" : "denied"))
        );

    // ── Lifecycle ──────────────────────────────────────────────

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        // Splash Screen — deve ser antes de super.onCreate
        SplashScreen splashScreen = SplashScreen.installSplashScreen(this);
        splashScreen.setKeepOnScreenCondition(() -> !isWebViewReady);

        super.onCreate(savedInstanceState);

        if (getSupportActionBar() != null) {
            getSupportActionBar().hide();
        }
        enableImmersiveMode();

        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);

        setContentView(R.layout.activity_main);

        webView = findViewById(R.id.webView);
        progressBar = findViewById(R.id.progressBar);
        offlineContainer = findViewById(R.id.offlineContainer);

        setupWebView();
        setupNetworkMonitor();
        setupBackNavigation();
        requestCameraPermissionIfNeeded();
        requestNotificationPermissionIfNeeded();

        if (isNetworkAvailable()) {
            loadMainUrl();
        } else {
            showOfflinePage();
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        isInForeground = true;
        enableImmersiveMode();
        if (isShowingOffline && isNetworkAvailable()) {
            hideOfflinePage();
            loadMainUrl();
        }
    }

    @Override
    protected void onPause() {
        super.onPause();
        isInForeground = false;
    }

    @Override
    protected void onDestroy() {
        if (networkCallback != null && connectivityManager != null) {
            try {
                connectivityManager.unregisterNetworkCallback(networkCallback);
            } catch (Exception e) {
                Log.w(TAG, "Error unregistering network callback", e);
            }
        }
        if (webView != null) {
            webView.stopLoading();
            webView.clearHistory();
            webView.removeAllViews();
            webView.destroy();
            webView = null;
        }
        super.onDestroy();
    }

    // ── WebView Setup ──────────────────────────────────────────

    @SuppressLint("SetJavaScriptEnabled")
    private void setupWebView() {
        WebSettings settings = webView.getSettings();

        // JavaScript + DOM Storage (HTMX, SSE, localStorage)
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setDatabaseEnabled(true);

        // Cache inteligente
        settings.setCacheMode(WebSettings.LOAD_DEFAULT);

        // Viewport (CSS controla via meta viewport)
        settings.setUseWideViewPort(true);
        settings.setLoadWithOverviewMode(true);
        settings.setSupportZoom(false);
        settings.setBuiltInZoomControls(false);
        settings.setDisplayZoomControls(false);

        // Encoding
        settings.setDefaultTextEncodingName("UTF-8");

        // Acesso a file:// para offline.html nos assets
        settings.setAllowFileAccess(true);

        // Forçar HTTPS — bloquear mixed content
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);

        // Media precisa de gesto do usuário
        settings.setMediaPlaybackRequiresUserGesture(true);

        // User-Agent customizado — identifica o app no backend
        String defaultUA = settings.getUserAgentString();
        settings.setUserAgentString(defaultUA + " DominoPlacarApp/1.0");

        // Cookies (sessão de mesa/jogador entre visitas)
        CookieManager cookieManager = CookieManager.getInstance();
        cookieManager.setAcceptCookie(true);
        cookieManager.setAcceptThirdPartyCookies(webView, true);

        // Background escuro durante carregamento
        webView.setBackgroundColor(0xFF09090F);

        // ── Bloqueia comportamento de "site" ───────────────────
        // Desabilita long-press (menu de contexto "copiar/selecionar")
        webView.setLongClickable(false);
        webView.setOnLongClickListener(v -> true);
        webView.setHapticFeedbackEnabled(false);

        // Desabilita scroll bars (parecer app nativo)
        webView.setVerticalScrollBarEnabled(false);
        webView.setHorizontalScrollBarEnabled(false);

        // Desabilita overscroll glow/bounce
        webView.setOverScrollMode(View.OVER_SCROLL_NEVER);

        // Clients
        webView.setWebViewClient(new DominoWebViewClient());
        webView.setWebChromeClient(new DominoWebChromeClient());
    }

    private void loadMainUrl() {
        if (webView == null) return;
        Map<String, String> headers = new HashMap<>();
        headers.put("Accept-Language", getAcceptLanguage());
        webView.loadUrl(BASE_URL, headers);
    }

    // ── WebViewClient ──────────────────────────────────────────

    private class DominoWebViewClient extends WebViewClient {

        // JS que transforma a WebView em experiência de app nativo.
        // Injeta <style> com !important para garantir que NADA seja selecionável,
        // e usa MutationObserver para cobrir conteúdo HTMX dinâmico.
        private static final String INJECT_APP_MODE_JS =
            "(function(){" +
                "if(document.getElementById('__app_webview_style'))return;" +
                "document.documentElement.classList.add('app-webview');" +
                // Injeta CSS diretamente com !important — mais forte que inline styles
                "var s=document.createElement('style');" +
                "s.id='__app_webview_style';" +
                "s.textContent='" +
                    "*:not(input):not(textarea){" +
                        "-webkit-user-select:none!important;" +
                        "user-select:none!important;" +
                        "-webkit-touch-callout:none!important;" +
                    "}" +
                    "input,textarea{" +
                        "-webkit-user-select:auto!important;" +
                        "user-select:auto!important;" +
                    "}" +
                    "*{-webkit-tap-highlight-color:transparent!important;}" +
                    "body{" +
                        "overscroll-behavior:contain!important;" +
                        "-webkit-overflow-scrolling:touch;" +
                    "}" +
                    "img,a{-webkit-touch-callout:none!important;}" +
                "';" +
                "(document.head||document.documentElement).appendChild(s);" +
                // Bloqueia menu de contexto
                "document.addEventListener('contextmenu',function(e){" +
                    "if(e.target.tagName!=='INPUT'&&e.target.tagName!=='TEXTAREA')e.preventDefault();" +
                "},true);" +
                // Bloqueia drag
                "document.addEventListener('dragstart',function(e){e.preventDefault();},true);" +
                // Bloqueia seleção via selectstart
                "document.addEventListener('selectstart',function(e){" +
                    "if(e.target.tagName!=='INPUT'&&e.target.tagName!=='TEXTAREA')e.preventDefault();" +
                "},true);" +
            "})();";

        @Override
        public void onPageStarted(WebView view, String url, android.graphics.Bitmap favicon) {
            super.onPageStarted(view, url, favicon);
            if (progressBar != null) {
                progressBar.setVisibility(View.VISIBLE);
            }
        }

        @Override
        public void onPageFinished(WebView view, String url) {
            super.onPageFinished(view, url);

            if (progressBar != null) {
                progressBar.setVisibility(View.GONE);
            }

            // Fecha splash screen
            if (!isWebViewReady) {
                isWebViewReady = true;
            }

            // Injeta novamente para garantir (conteúdo pode ter mudado)
            view.evaluateJavascript(INJECT_APP_MODE_JS, null);

            // Detecta matchID na URL e registra para push notifications
            extractAndRegisterMatch(url);
        }

        @Override
        public void onPageCommitVisible(WebView view, String url) {
            super.onPageCommitVisible(view, url);
            // Terceira injeção — quando o conteúdo fica visível
            view.evaluateJavascript(INJECT_APP_MODE_JS, null);
        }

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
            Uri uri = request.getUrl();
            String host = uri.getHost();

            // Navegação interna
            if (host != null && host.endsWith(ALLOWED_HOST)) {
                Map<String, String> headers = new HashMap<>();
                headers.put("Accept-Language", getAcceptLanguage());
                view.loadUrl(uri.toString(), headers);
                return true;
            }

            // Links externos → navegador do sistema
            try {
                Intent intent = new Intent(Intent.ACTION_VIEW, uri);
                intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                startActivity(intent);
            } catch (ActivityNotFoundException e) {
                Log.w(TAG, "No browser to open: " + uri);
            }
            return true;
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
            super.onReceivedError(view, request, error);
            if (request.isForMainFrame()) {
                Log.e(TAG, "WebView error: " + error.getDescription());
                showOfflinePage();
            }
        }

        @Override
        public void onReceivedHttpError(WebView view, WebResourceRequest request,
                                        android.webkit.WebResourceResponse errorResponse) {
            super.onReceivedHttpError(view, request, errorResponse);
            if (request.isForMainFrame() && errorResponse.getStatusCode() >= 500) {
                Log.e(TAG, "HTTP " + errorResponse.getStatusCode() + " on " + request.getUrl());
            }
        }
    }

    // ── WebChromeClient ────────────────────────────────────────

    private class DominoWebChromeClient extends WebChromeClient {

        /**
         * CRÍTICO: Sem este override, <input type="file"> NÃO funciona na WebView.
         * Abre chooser que combina câmera + galeria + file picker.
         */
        @Override
        public boolean onShowFileChooser(WebView webView,
                                         ValueCallback<Uri[]> filePathCallback,
                                         FileChooserParams fileChooserParams) {
            // Cancela callback anterior (evita leak)
            if (fileUploadCallback != null) {
                fileUploadCallback.onReceiveValue(null);
            }
            fileUploadCallback = filePathCallback;

            // Intent da câmera
            Intent cameraIntent = createCameraIntent();

            // Intent do file chooser
            Intent chooserIntent;
            if (fileChooserParams != null) {
                chooserIntent = fileChooserParams.createIntent();
            } else {
                chooserIntent = new Intent(Intent.ACTION_GET_CONTENT);
                chooserIntent.setType("image/*");
            }

            Intent selectorIntent = new Intent(Intent.ACTION_CHOOSER);
            selectorIntent.putExtra(Intent.EXTRA_INTENT, chooserIntent);
            if (cameraIntent != null) {
                selectorIntent.putExtra(Intent.EXTRA_INITIAL_INTENTS, new Intent[]{cameraIntent});
            }
            selectorIntent.putExtra(Intent.EXTRA_TITLE, getString(R.string.file_chooser_title));

            try {
                fileChooserLauncher.launch(selectorIntent);
            } catch (ActivityNotFoundException e) {
                Log.e(TAG, "No file chooser available", e);
                fileUploadCallback.onReceiveValue(null);
                fileUploadCallback = null;
                return false;
            }
            return true;
        }

        @Override
        public void onProgressChanged(WebView view, int newProgress) {
            if (progressBar != null) {
                progressBar.setProgress(newProgress);
                progressBar.setVisibility(newProgress < 100 ? View.VISIBLE : View.GONE);
            }
        }

        @Override
        public void onReceivedTitle(WebView view, String title) {
            super.onReceivedTitle(view, title);
            Log.d(TAG, "Page: " + title);
        }

        @Override
        public void onPermissionRequest(PermissionRequest request) {
            runOnUiThread(() -> request.grant(request.getResources()));
        }
    }

    // ── Camera / File Upload ───────────────────────────────────

    private Intent createCameraIntent() {
        if (!hasCameraPermission()) return null;

        Intent cameraIntent = new Intent(MediaStore.ACTION_IMAGE_CAPTURE);
        if (cameraIntent.resolveActivity(getPackageManager()) == null) return null;

        try {
            File photoFile = createImageFile();
            cameraPhotoUri = FileProvider.getUriForFile(
                this,
                getApplicationContext().getPackageName() + ".fileprovider",
                photoFile
            );
            cameraIntent.putExtra(MediaStore.EXTRA_OUTPUT, cameraPhotoUri);
            cameraIntent.addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
            return cameraIntent;
        } catch (IOException e) {
            Log.e(TAG, "Error creating camera file", e);
            return null;
        }
    }

    private File createImageFile() throws IOException {
        String timeStamp = new SimpleDateFormat("yyyyMMdd_HHmmss", Locale.US).format(new Date());
        File storageDir = getExternalCacheDir();
        if (storageDir == null) storageDir = getCacheDir();
        return File.createTempFile("DOMINO_" + timeStamp + "_", ".jpg", storageDir);
    }

    private boolean hasCameraPermission() {
        return ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA)
            == PackageManager.PERMISSION_GRANTED;
    }

    private void requestCameraPermissionIfNeeded() {
        if (!hasCameraPermission()) {
            cameraPermissionLauncher.launch(Manifest.permission.CAMERA);
        }
    }

    private void requestNotificationPermissionIfNeeded() {
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
                    != PackageManager.PERMISSION_GRANTED) {
                notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS);
            }
        }
    }

    // ── Offline Handling ───────────────────────────────────────

    private void showOfflinePage() {
        isShowingOffline = true;
        runOnUiThread(() -> {
            if (webView != null) webView.setVisibility(View.GONE);
            if (offlineContainer != null) offlineContainer.setVisibility(View.VISIBLE);
            if (progressBar != null) progressBar.setVisibility(View.GONE);
            if (!isWebViewReady) isWebViewReady = true;
        });
    }

    private void hideOfflinePage() {
        isShowingOffline = false;
        runOnUiThread(() -> {
            if (offlineContainer != null) offlineContainer.setVisibility(View.GONE);
            if (webView != null) webView.setVisibility(View.VISIBLE);
        });
    }

    /** Chamado pelo botão "Tentar Novamente" do layout offline. */
    public void onRetryClicked(View view) {
        if (isNetworkAvailable()) {
            hideOfflinePage();
            loadMainUrl();
        } else {
            Toast.makeText(this, getString(R.string.error_no_internet), Toast.LENGTH_SHORT).show();
        }
    }

    // ── Network Monitoring ─────────────────────────────────────

    private void setupNetworkMonitor() {
        connectivityManager = (ConnectivityManager) getSystemService(Context.CONNECTIVITY_SERVICE);
        if (connectivityManager == null) return;

        networkCallback = new ConnectivityManager.NetworkCallback() {
            @Override
            public void onAvailable(@NonNull Network network) {
                isConnected = true;
                runOnUiThread(() -> {
                    if (isShowingOffline && webView != null) {
                        hideOfflinePage();
                        loadMainUrl();
                    }
                });
            }

            @Override
            public void onLost(@NonNull Network network) {
                isConnected = false;
                runOnUiThread(() ->
                    Toast.makeText(MainActivity.this,
                        getString(R.string.error_no_internet), Toast.LENGTH_SHORT).show()
                );
            }

            @Override
            public void onCapabilitiesChanged(@NonNull Network network,
                                              @NonNull NetworkCapabilities caps) {
                isConnected = caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                    && caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED);
            }
        };

        NetworkRequest request = new NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .build();
        connectivityManager.registerNetworkCallback(request, networkCallback);
    }

    private boolean isNetworkAvailable() {
        if (connectivityManager == null) return true;
        NetworkCapabilities caps = connectivityManager.getNetworkCapabilities(
            connectivityManager.getActiveNetwork()
        );
        return caps != null
            && caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            && caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED);
    }

    // ── Immersive Mode ─────────────────────────────────────────

    private void enableImmersiveMode() {
        WindowCompat.setDecorFitsSystemWindows(getWindow(), false);
        WindowInsetsControllerCompat controller = WindowCompat.getInsetsController(
            getWindow(), getWindow().getDecorView()
        );
        if (controller != null) {
            controller.hide(WindowInsetsCompat.Type.systemBars());
            controller.setSystemBarsBehavior(
                WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
            );
        }
    }

    // ── Back Navigation ────────────────────────────────────────

    private void setupBackNavigation() {
        getOnBackPressedDispatcher().addCallback(this, new OnBackPressedCallback(true) {
            @Override
            public void handleOnBackPressed() {
                if (isShowingOffline) {
                    finish();
                    return;
                }
                if (webView != null && webView.canGoBack()) {
                    webView.goBack();
                } else {
                    finish();
                }
            }
        });
    }

    // ── i18n ───────────────────────────────────────────────────

    private String getAcceptLanguage() {
        Locale locale = Locale.getDefault();
        String langTag = locale.toLanguageTag();
        String baseLang = locale.getLanguage();
        if (baseLang.equals(langTag)) {
            return langTag + ",en;q=0.8";
        }
        return langTag + "," + baseLang + ";q=0.9,en;q=0.8";
    }

    // ── Push Notification Registration ─────────────────────────

    /**
     * Extrai matchID da URL e registra o device para push notifications.
     * URLs como /match/{id}/game, /match/{id}/lobby, etc.
     */
    private void extractAndRegisterMatch(String url) {
        if (url == null) return;
        // Regex: /match/{uuid ou id}/
        java.util.regex.Matcher m = java.util.regex.Pattern
            .compile("/match/([a-zA-Z0-9-]+)")
            .matcher(url);
        if (m.find()) {
            String matchId = m.group(1);
            if (matchId != null && !matchId.equals(currentMatchId)) {
                currentMatchId = matchId;
                Log.d(TAG, "Registering push for match: " + matchId);
                DominoPushService.registerForMatch(this, matchId);
            }
        } else {
            // Não está em uma partida
            currentMatchId = null;
        }
    }
}
