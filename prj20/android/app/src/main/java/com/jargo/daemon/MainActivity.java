package com.jargo.daemon;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.widget.Button;
import android.widget.TextView;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;

public class MainActivity extends Activity {
    private TextView tvStatus;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        tvStatus = findViewById(R.id.tvStatus);
        Button btnUpload = findViewById(R.id.btnUpload);

        btnUpload.setOnClickListener(v -> {
            Intent intent = new Intent(Intent.ACTION_GET_CONTENT);
            intent.setType("*/*"); // Mengizinkan semua file binary tanpa ekstensi
            startActivityForResult(intent, 1);
        });
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode == 1 && resultCode == RESULT_OK && data != null) {
            injectAndRunBinary(data.getData());
        }
    }

    private void injectAndRunBinary(Uri uri) {
        try {
            // Karena SAF tidak memberi nama asli dengan mudah, kita generate nama unik
            String binName = "engine_" + System.currentTimeMillis();
            File destFile = new File(getFilesDir(), binName);

            InputStream in = getContentResolver().openInputStream(uri);
            FileOutputStream out = new FileOutputStream(destFile);
            
            byte[] buffer = new byte[8192];
            int read;
            while ((read = in.read(buffer)) != -1) {
                out.write(buffer, 0, read);
            }
            in.close();
            out.close();

            tvStatus.setText("Biner disuntikkan: " + binName + "\nMengeksekusi...");

            // Perintahkan Service untuk menjalankan biner ini
            Intent serviceIntent = new Intent(this, DaemonService.class);
            serviceIntent.setAction("START_BIN");
            serviceIntent.putExtra("BIN_NAME", binName);
            startService(serviceIntent);

        } catch (Exception e) {
            tvStatus.setText("Gagal injeksi: " + e.getMessage());
        }
    }
}
