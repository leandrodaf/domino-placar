package net.dominoplacar.app;

import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Intent;
import android.content.SharedPreferences;
import android.media.RingtoneManager;
import android.os.Build;
import android.util.Log;

import androidx.annotation.NonNull;
import androidx.core.app.NotificationCompat;

import com.google.firebase.messaging.FirebaseMessagingService;
import com.google.firebase.messaging.RemoteMessage;

import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.Map;

/**
 * Serviço FCM para push notifications do Dominó Placar.
 *
 * Recebe notificações do backend Go quando eventos acontecem na partida
 * (rodada começou, pontuação atualizada, partida finalizada, etc.)
 * mesmo quando o app está em background ou fechado.
 */
public class DominoPushService extends FirebaseMessagingService {

    private static final String TAG = "DominoPush";
    private static final String CHANNEL_ID = "domino_match_events";
    private static final String PREFS_NAME = "domino_push";
    private static final String KEY_FCM_TOKEN = "fcm_token";
    private static final String BASE_URL = "https://dominoplacar.net";

    // ── Token Management ───────────────────────────────────────

    @Override
    public void onNewToken(@NonNull String token) {
        super.onNewToken(token);
        Log.d(TAG, "New FCM token: " + token);

        // Salva localmente
        getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
            .edit()
            .putString(KEY_FCM_TOKEN, token)
            .apply();

        // Envia para o backend
        sendTokenToServer(token);
    }

    /**
     * Envia o token FCM para o backend Go para que ele possa enviar push.
     * Chamado quando o token é gerado/renovado e quando o jogador entra em uma partida.
     */
    private void sendTokenToServer(String token) {
        new Thread(() -> {
            try {
                // Verifica se tem matchID ativo
                String matchId = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
                    .getString("active_match_id", null);

                if (matchId == null || matchId.isEmpty()) {
                    Log.d(TAG, "No active match, skipping token registration");
                    return;
                }

                URL url = new URL(BASE_URL + "/api/push/register");
                HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                conn.setRequestMethod("POST");
                conn.setRequestProperty("Content-Type", "application/json");
                conn.setDoOutput(true);
                conn.setConnectTimeout(10000);
                conn.setReadTimeout(10000);

                String json = "{\"token\":\"" + escapeJson(token) +
                    "\",\"match_id\":\"" + escapeJson(matchId) + "\"}";

                try (OutputStream os = conn.getOutputStream()) {
                    os.write(json.getBytes(StandardCharsets.UTF_8));
                }

                int code = conn.getResponseCode();
                Log.d(TAG, "Token registration response: " + code);
                conn.disconnect();
            } catch (Exception e) {
                Log.e(TAG, "Failed to send token to server", e);
            }
        }).start();
    }

    /**
     * Registra o token para uma partida específica.
     * Chamado pela MainActivity quando o jogador acessa uma partida.
     */
    public static void registerForMatch(android.content.Context context, String matchId) {
        SharedPreferences prefs = context.getSharedPreferences(PREFS_NAME, MODE_PRIVATE);
        prefs.edit().putString("active_match_id", matchId).apply();

        String token = prefs.getString(KEY_FCM_TOKEN, null);
        if (token != null) {
            new Thread(() -> {
                try {
                    URL url = new URL(BASE_URL + "/api/push/register");
                    HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                    conn.setRequestMethod("POST");
                    conn.setRequestProperty("Content-Type", "application/json");
                    conn.setDoOutput(true);
                    conn.setConnectTimeout(10000);
                    conn.setReadTimeout(10000);

                    String json = "{\"token\":\"" + escapeJson(token) +
                        "\",\"match_id\":\"" + escapeJson(matchId) + "\"}";

                    try (OutputStream os = conn.getOutputStream()) {
                        os.write(json.getBytes(StandardCharsets.UTF_8));
                    }

                    int code = conn.getResponseCode();
                    Log.d(TAG, "Match registration response: " + code);
                    conn.disconnect();
                } catch (Exception e) {
                    Log.e(TAG, "Failed to register for match", e);
                }
            }).start();
        }
    }

    // ── Message Handling ───────────────────────────────────────

    @Override
    public void onMessageReceived(@NonNull RemoteMessage remoteMessage) {
        super.onMessageReceived(remoteMessage);
        Log.d(TAG, "Push received from: " + remoteMessage.getFrom());

        Map<String, String> data = remoteMessage.getData();
        String eventType = data.getOrDefault("event", "");
        String matchId = data.getOrDefault("match_id", "");
        String title = data.getOrDefault("title", getString(R.string.app_name));
        String body = data.getOrDefault("body", "");

        // Se o app está em foreground com a mesma partida, o SSE já cuida — ignora
        if (isAppInForeground(matchId)) {
            Log.d(TAG, "App in foreground for match " + matchId + ", skipping notification");
            return;
        }

        if (!body.isEmpty()) {
            showNotification(title, body, matchId, eventType);
        }
    }

    private boolean isAppInForeground(String matchId) {
        // Checa se a MainActivity está ativa e visível
        return MainActivity.isInForeground && matchId.equals(MainActivity.currentMatchId);
    }

    // ── Notification Display ───────────────────────────────────

    private void showNotification(String title, String body, String matchId, String eventType) {
        createNotificationChannel();

        // Intent que abre o app na partida correta
        Intent intent = new Intent(this, MainActivity.class);
        intent.addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP);
        if (matchId != null && !matchId.isEmpty()) {
            intent.setData(android.net.Uri.parse(BASE_URL + "/match/" + matchId + "/game"));
        }

        PendingIntent pendingIntent = PendingIntent.getActivity(
            this, matchId.hashCode(), intent,
            PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE
        );

        // Ícone de notificação baseado no tipo de evento
        int icon = getNotificationIcon(eventType);

        NotificationCompat.Builder builder = new NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(icon)
            .setContentTitle(title)
            .setContentText(body)
            .setAutoCancel(true)
            .setSound(RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION))
            .setContentIntent(pendingIntent)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setCategory(NotificationCompat.CATEGORY_EVENT);

        NotificationManager manager =
            (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        if (manager != null) {
            // Usa matchId hash como notification ID para agrupar por partida
            manager.notify(matchId.hashCode(), builder.build());
        }
    }

    private int getNotificationIcon(String eventType) {
        // Usa o ícone do app — funciona em todas as versões
        return R.drawable.ic_launcher_foreground;
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                CHANNEL_ID,
                "Eventos da Partida",
                NotificationManager.IMPORTANCE_HIGH
            );
            channel.setDescription("Notificações de rodadas, pontuação e resultados");
            channel.enableVibration(true);

            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager != null) {
                manager.createNotificationChannel(channel);
            }
        }
    }

    // ── Util ───────────────────────────────────────────────────

    private static String escapeJson(String value) {
        if (value == null) return "";
        return value.replace("\\", "\\\\").replace("\"", "\\\"");
    }
}
