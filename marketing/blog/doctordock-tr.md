---
title: "Docker'ıma Bir Doktor Yazdım"
subtitle: "Çünkü aylardır makinemde ne döndüğünü tam olarak bilmiyordum"
description: "Docker'ı her gün kullanıyordum ama içinde ne olduğunu bilmiyordum. DoctorDock, yerel Docker ortamını güvenlik, hatalı yapılandırma ve boşa giden disk için saniyenin altında, çevrimdışı tarayan açık kaynak bir araç."
date: 2026-08-14
author: "Can Türk"
lang: tr
tags: [docker, güvenlik, devops, açık-kaynak, cli]
canonical: "https://doctordock.iamcanturk.dev"
---

# Docker'ıma Bir Doktor Yazdım

Bir akşam, uzun bir günün sonunda `docker ps` yazdım ve çıktıya bir süre öylece baktım. Yirmi altı konteyner. Bir kısmının ne için ayakta olduğunu hatırlıyordum; geri kalanı, aylar önce "bir dakika, şunu bir deneyeyim" deyip unuttuğum şeylerdi. Sonra `docker images` yazdım: on üç gigabayt. Kimsenin çekmediği katmanlar, çoktan gereksizleşmiş imajlar.

O an rahatsız edici bir şeyi kabul ettim: Docker'ı her gün kullanıyordum ama içinde tam olarak ne döndüğünü bilmiyordum.

Docker bu konuda tuhaf bir araç. Bir şeyi çalıştırmayı olağanüstü kolaylaştırıyor, ne çalıştırdığını fark etmeyi ise aynı ölçüde zorlaştırıyor. Altı ay süren bir projenin sonunda ortalama bir geliştiricinin makinesinde şunlar birikiyor: Docker soketini içine bağlamış bir konteyner, `0.0.0.0` üzerinde herkese açık birkaç veritabanı, root olarak çalışan bir düzine servis ve kimsenin referans vermediği gigabaytlarca imaj. Hiçbiri o an alarm çalmıyor — ta ki bir şey bozulana ya da biri seni taramaya karar verene kadar.

![DoctorDock sağlık kartı: 100 üzerinden Docker sağlık puanı ve kaynak sayıları](https://doctordock.iamcanturk.dev/assets/health-card.png)

## Var olan araçlar bu boşluğu doldurmuyor

Yanlış anlaşılmasın, iyi araçlar var. Trivy ve Grype imajlardaki CVE'leri gayet iyi buluyor. Ama onlar bir zafiyet veritabanına, dolayısıyla ağ bağlantısına ihtiyaç duyuyor ve baktıkları şey imajın *içindeki* paketler. Benim sorunum orada değildi. Benim sorunum **yapılandırma katmanındaydı**: o konteyneri nasıl çalıştırdığım. `docker system df` disk kullanımını söylüyor ama hiçbir yorum yapmıyor; "şu volume'a artık kimse dokunmuyor" demiyor.

Aradığım şey tek bir soruya cevap veren bir araçtı: **Bu Docker ortamında şu an neyin yanlış olduğu ve ne yapmam gerektiği.** Bulamayınca yazdım.

## DoctorDock

DoctorDock, yerel Docker ortamını tarayan küçük bir komut satırı aracı — yanında bir de native macOS menubar uygulaması. Güvenlik problemlerini, hatalı yapılandırmaları ve geri kazanılabilir diski bulup önüne koyuyor, sonuna da 100 üzerinden bir sağlık puanı bırakıyor. Go ile yazdım; tek binary, hesap yok, MIT lisansı.

Hız konusunda somut olayım: bende yaklaşık 26 konteyner, 29 imaj, 29 volume ve 12 ağdan oluşan bir ortamın tam taraması ~550 ms sürüyor. `docker ps` yazıp çıktısını okuma sürenden kısa.

## Verdiğim üç karar

Bu aracı yazarken üç şeye baştan karar verdim. Üçü de "eksik özellik" gibi görünebilir; bence tam tersi.

### Yapay zeka yok

Bunu bir "AI destekli güvenlik ürünü" yapabilirdim. Yapmadım. Her bulgu, okuyabileceğin deterministik Go kodu; aynı ortam her zaman aynı çıktıyı üretiyor. Bir kuralın neyi neden işaretlediğini merak edersen kaynağına bakabilirsin. Bir güvenlik aracının bana "sanırım burada bir sorun olabilir" demesini istemiyorum — ya vardır, ya yoktur.

### Çevrimdışı

Sıfır ağ çağrısı. Telemetri yok, hesap yok, güncelleme kontrolü yok. DoctorDock yalnızca tek bir yerel soketi açıyor: Docker'ınkini, başka hiçbir şeyi. Air-gapped bir makinede de, kapalı bir CI ortamında da aynı şekilde çalışıyor. Ne taradığın sende kalıyor.

### Sırlar yerinde kalır

Konteyner ortam değişkenlerini yalnızca **anahtar adı** olarak okuyorum; değerleri bir rapora ulaşabilecek hiçbir biçimde belleğe girmiyor. `DATABASE_PASSWORD` diye bir anahtarın olduğunu görürsün, değerini asla. Bu da aracı production'a karşı çalıştırmayı güvenli kılıyor — ki bunun bir güvenlik aracında pazarlanacak bir özellik değil, varsayılan olması gerekir.

## Nasıl görünüyor

Kurup taramak dışında bir şey yapman gerekmiyor:

```bash
brew install iamcanturk/tap/doctordock
doctordock scan
```

Çıktı ortama bir puan veriyor ve bulguları önem sırasına diziyor:

```
HEALTH SCORE  37/100  poor

  1 CRITICAL · 17 HIGH · 6 MEDIUM · 22 LOW

  DD005  CRITICAL  Docker socket bir konteynere mount edilmiş
  DD001  HIGH      Konteyner root olarak çalışıyor
  DD006  MEDIUM    Veritabanı portu 0.0.0.0 üzerinde yayınlanmış
```

37/100 gördüğünde paniğe gerek yok; çoğu geliştirici makinesi ilk taramada buralardan başlıyor. Önemli olan artık listenin görünür olması. Ve her kural kendini anlatıyor:

```bash
doctordock explain DD005
```

Bu komut sana kuralın ne aradığını, neden önemli olduğunu, somut bir saldırı senaryosunu, kopyalayıp yapıştırabileceğin bir çözümü ve ne zaman güvenle görmezden gelinebileceğini veriyor. "Burada bir sorun var" demekle yetinmiyor, o sorunu kapatmayı da öğretiyor.

![Terminalde DoctorDock: komut listesi ve doctordock explain DD005 çıktısı](https://doctordock.iamcanturk.dev/assets/demo.gif)

## En çok üzerinde durduğum kısım: temizlik

Açıkçası bu aracı yazarken en çok temizlik kısmından çekindim. Çünkü yanlış bir komut, birinin aylık verisini uçurabilir. O yüzden DoctorDock varsayılan olarak hiçbir şey silmiyor:

```bash
doctordock cleanup            # kuru çalışma: sadece ne silineceğini gösterir
doctordock cleanup --apply    # gerçekten uygular
```

Ve `--volumes` dışında hiçbir flag bir volume'u seçemiyor — `--all` bile. Bir imajı yeniden çekebilir, bir ağı yeniden kurabilirsin; ama bir volume'daki veri gittiğinde geri gelmiyor. Bu yüzden volume'lar ancak açıkça ve tek başına istediğinde işin içine giriyor. Kazara hiçbir şey silinmiyor.

## Kapanış

DoctorDock büyük bir ürün değil; küçük ve dürüst bir araç. Docker ortamının aylar içinde sessizce biriktirdiği şeyleri saniyenin altında, hiçbir veriyi dışarı sızdırmadan önüne koyuyor. Bir doktor gibi: bakıyor, teşhis koyuyor, ne yapman gerektiğini söylüyor ve kararı sana bırakıyor.

Merak ettiysen bir tarama çalıştır ve makinenin gerçekte kaç aldığını gör. Muhtemelen sandığından düşük çıkacak — ama ilk kez tam olarak ne yapacağını bileceksin.

- Kod: [github.com/iamcanturk/DoctorDock](https://github.com/iamcanturk/DoctorDock)
- İndir ve detaylar: [doctordock.iamcanturk.dev](https://doctordock.iamcanturk.dev)
