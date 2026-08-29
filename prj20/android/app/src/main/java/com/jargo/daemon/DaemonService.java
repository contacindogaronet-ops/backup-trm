package com.jargo.daemon;

import android.app.Service;
import android.content.Intent;
import android.os.IBinder;
import android.util.Log;
import java.io.BufferedReader;
import java.io.File;
import java.io.InputStreamReader;
import java.util.HashMap;
import java.util.Map;

public class DaemonService extends Service {
    // Matriks penyimpanan proses yang berjalan
    private final Map<String, Process> activeProcesses = new HashMap<>();

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null) {
            String action = intent.getAction();
            String binName = intent.getStringExtra("BIN_NAME");

            if ("START_BIN".equals(action) && binName != null) {
                executeBinary(binName);
            } else if ("STOP_BIN".equals(action) && binName != null) {
                killBinary(binName);
            }
        }
        return START_STICKY;
    }

    private void executeBinary(String binName) {
        if (activeProcesses.containsKey(binName)) return; // Sudah berjalan

        File binFile = new File(getFilesDir(), binName);
        if (!binFile.exists()) {
            Log.e("DAEMON", "Biner tidak ditemukan: " + binName);
            return;
        }

        // 🔴 KUNCI ARSITEKTUR: Mutlak butuh hak eksekusi (chmod +x)
        binFile.setExecutable(true, false);

        new Thread(() -> {
            try {
                ProcessBuilder pb = new ProcessBuilder(binFile.getAbsolutePath());
                pb.directory(getFilesDir()); // Jalankan di direktori privat
                pb.redirectErrorStream(true); // Gabung stdout dan stderr
                
                Process process = pb.start();
                activeProcesses.put(binName, process);
                Log.i("DAEMON", "🚀 Biner [ " + binName + " ] berhasil dihidupkan.");

                // Tarik log dari biner secara real-time
                BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
                String line;
                while ((line = reader.readLine()) != null) {
                    Log.d("BIN_" + binName, line); // Akan dicetak ke Logcat
                }
                
                process.waitFor();
                activeProcesses.remove(binName);
                Log.w("DAEMON", "💀 Biner [ " + binName + " ] telah mati/berhenti.");

            } catch (Exception e) {
                Log.e("DAEMON", "Gagal mengeksekusi " + binName, e);
            }
        }).start();
    }

    private void killBinary(String binName) {
        Process process = activeProcesses.get(binName);
        if (process != null) {
            process.destroy(); // Kirim sinyal SIGTERM
            activeProcesses.remove(binName);
            Log.i("DAEMON", "Biner [ " + binName + " ] dihentikan paksa.");
        }
    }

    @Override
    public IBinder onBind(Intent intent) { return null; }

    @Override
    public void onDestroy() {
        super.onDestroy();
        // Bantai semua zombie process saat service dimatikan
        for (Process p : activeProcesses.values()) {
            if (p != null) p.destroy();
        }
        activeProcesses.clear();
    }
}
