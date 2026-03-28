# ProGuard rules for Dominó Placar
# ─────────────────────────────────────────

# Manter a MainActivity e inner classes
-keep class net.dominoplacar.app.** { *; }

# WebView JavaScript interface (caso adicione @JavascriptInterface no futuro)
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

# WebView — não ofuscar classes usadas pelo WebView
-keepclassmembers class * extends android.webkit.WebViewClient { *; }
-keepclassmembers class * extends android.webkit.WebChromeClient { *; }

# FileProvider
-keep class androidx.core.content.FileProvider { *; }

# Splash Screen
-keep class androidx.core.splashscreen.** { *; }

# Remover logs em release
-assumenosideeffects class android.util.Log {
    public static boolean isLoggable(java.lang.String, int);
    public static int v(...);
    public static int d(...);
    public static int i(...);
}

# Otimizações gerais
-optimizationpasses 5
-dontusemixedcaseclassnames
-verbose
