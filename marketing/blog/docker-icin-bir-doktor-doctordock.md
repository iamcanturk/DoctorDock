---
title: "DoctorDock: Docker ortamınız için çevrimdışı bir doktor"
description: "DoctorDock, yerel Docker ortamınızı saniyeden kısa sürede, tamamen çevrimdışı tarayan açık kaynak bir CLI. Güvenlik açıkları, yapılandırma hataları ve boşa giden disk tek komutta."
date: 2026-08-14
author: "Can Türk"
tags: [docker, güvenlik, devops, açık-kaynak, cli]
canonical: "https://doctordock.iamcanturk.dev"
---

# DoctorDock: Docker'ınız için bir doktor

Docker'la çalışan herkesin makinesinde zamanla sessizce biriken bir tortu vardır. Bir akşam hızlıca ayağa kaldırdığınız `--privileged` konteyner. Test için `0.0.0.0` üzerinde açtığınız ve kapatmayı unuttuğunuz bir Postgres portu. Üç ay önce denediğiniz bir imajın artık kimsenin çekmediği katmanları. Silinmemiş volume'lar, restart döngüsünde dönüp duran bir servis, `~/.aws`'i konteynerin içine mount eden bir compose satırı.

Bunların hiçbiri o an alarm çalmaz. Kod derlenir, uygulama açılır, iş yürür. Ama bu küçük kararlar aylar içinde birikir ve bir gün ya diskiniz dolar ya da güvenlik gözden geçirmesinde birinin gözüne batar. İşte **DoctorDock** tam bu görünmez tortuyu görünür kılmak için yazıldı.

![DoctorDock sağlık kartı: 100 üzerinden Docker sağlık puanı ve kaynak sayıları](https://doctordock.iamcanturk.dev/assets/health-card.png)
*DoctorDock'un paylaşılabilir sağlık kartı — yalnızca toplam sayılar, hiçbir konteyner adı veya port sızmaz.*

## Problem: kimse Docker yapılandırmasına bakmıyor

CVE taraması artık çözülmüş bir problem. Trivy ve Grype imajlarınızdaki paket zafiyetlerini gayet iyi buluyor. Ama bir zafiyet veritabanına ve dolayısıyla ağ bağlantısına ihtiyaç duyuyorlar.

Asıl boşluk **yapılandırma katmanında**. Konteynerin root olarak mı çalıştığı, socket'in mount edilip edilmediği, hangi portun dışarı açıldığı, healthcheck'in olup olmadığı. Bunlar imajın içindeki bir paketin sürümüyle ilgili değil, o konteyneri *nasıl çalıştırdığınızla* ilgili. Ve bu katmana kimse bakmıyor.

Mevcut alternatifler de pek yardımcı olmuyor. Docker Bench beş bin satırlık bir shell script. `docker system df` size disk kullanımını söyler ama hiçbir yorum yapmaz, "şu volume gereksiz" demez. DoctorDock tek bir soruya cevap vermek için var: **"Bu Docker ortamında şu an neyin yanlış olduğu ve ne yapmam gerektiği?"**

## DoctorDock nedir

DoctorDock, yerel Docker ortamınızı tarayan **yerel öncelikli (local-first)** bir komut satırı aracı. Yanında native bir macOS menubar uygulaması da geliyor. Güvenlik problemlerini, yapılandırma hatalarını ve geri kazanılabilir diski (boşa giden kaynakları) bulup önünüze koyar. Saniyenin altında, tamamen çevrimdışı.

![DoctorDock macOS uygulaması: sağlık puanı, kaynak sayıları ve önce neyi düzeltmeli](https://doctordock.iamcanturk.dev/assets/app-overview.png)
*Native macOS menubar uygulaması — CLI ile birebir aynı motoru kullanır.*

Go ile yazıldı, tek binary. Hesap açmanız gerekmiyor, MIT lisansıyla açık kaynak. Hem `doctordock` hem de kısa takma adı `ddock` kuruluyor.

Ne kadar hızlı olduğunu somutlaştırayım: yaklaşık 26 konteyner, 29 imaj, 29 volume ve 12 ağdan oluşan bir ortamın tam taraması bende yaklaşık **550 ms** sürüyor. Yani `docker ps` yazıp çıktısına bakacağınız süreden daha kısa.

## Neden farklı: AI yok, ağ yok, telemetri yok

Bu araç bir "AI-powered" güvenlik ürünü değil. Bunu bir eksiklik değil, tasarım kararı olarak söylüyorum.

### Yapay zeka yok

Her bulgu, okuyabileceğiniz deterministik Go kodu. Aynı ortam her zaman aynı çıktıyı üretir. Halüsinasyon yok, "bugün böyle dedi yarın başka türlü" yok. Bir kuralın neyi neden işaretlediğini merak ederseniz kaynağına bakabilirsiniz. İleride AI bu işin içine girerse görevi yalnızca sonuçları *açıklamak* olur, sonuçları *üretmek* asla.

### Tamamen çevrimdışı

Sıfır ağ çağrısı. Senkronize edilecek bir CVE beslemesi yok, hesap yok, güncelleme kontrolü yok. Air-gapped bir makinede de, kilitli bir CI ortamında da aynı şekilde çalışır. DoctorDock yalnızca tek bir yerel soketi açar, Docker'ınkini, başka hiçbir şeyi.

### Telemetri yok

Hiçbir şey toplanmıyor, iletilmiyor, eve telefon açılmıyor. Ne taradığınız sizde kalır.

### Sırlar yerinde kalır

Konteyner ortam değişkenleri yalnızca **anahtar adı** olarak okunur, değerleri bir rapora ulaşabilecek hiçbir biçimde belleğe girmez. Bu da DoctorDock'u production'a karşı çalıştırmayı güvenli kılar. `DATABASE_PASSWORD` diye bir anahtarın var olduğunu görürsünüz ama değerini asla.

Bu dört madde bir araya gelince ortaya net bir konumlanma çıkıyor: **Docker güvenliği** ve **Docker yapılandırma hataları** için, gizliliğinizi hiç zorlamadan çalışan bir **yerel Docker analizi** aracı.

## Nasıl kurulur

macOS'ta Homebrew ile:

```bash
brew install iamcanturk/tap/doctordock
```

Her platformda Go ile:

```bash
go install github.com/iamcanturk/DoctorDock/cmd/doctordock@latest
```

CI veya konteyner içinde, socket'i salt okunur bağlayarak:

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock:ro ghcr.io/iamcanturk/doctordock
```

Desteklenen platformlar macOS ve Linux (ayrıca Go ve konteyner üzerinden her yerde). Kurulduğunda hem `doctordock` hem `ddock` elinizin altında olur.

## Örnek çıktı

Kurduktan sonra tek yapmanız gereken taramak. DoctorDock ortamınıza bir sağlık puanı verir (100 üzerinden) ve bulguları önem sırasına göre listeler:

```
HEALTH SCORE  37/100  poor

  1 CRITICAL · 17 HIGH · 6 MEDIUM · 22 LOW

  DD005  CRITICAL  Docker socket bir konteynere mount edilmiş
  DD001  HIGH      Konteyner root olarak çalışıyor
  DD006  MEDIUM    Veritabanı portu 0.0.0.0 üzerinde yayınlanmış
  DD007  LOW       Healthcheck tanımlı değil
  DD015  INFO      Kullanılmayan imaj
```

37/100 gördüğünüzde paniğe gerek yok, çoğu geliştirici makinesi ilk taramada buna yakın bir yerden başlıyor. Önemli olan artık listenin görünür olması ve her satırın ne anlama geldiğini tam olarak açıklayabilmesi.

Herhangi bir kural kendini anlatır:

```bash
doctordock explain DD005
```

Bu komut size o kuralın ne aradığını, neden önemli olduğunu, somut bir saldırı senaryosunu, kopyalayıp yapıştırabileceğiniz bir çözümü, ne zaman güvenle görmezden gelinebileceğini ve konuyla ilgili kaynak linklerini verir. Yani sadece "burada bir sorun var" demiyor, o sorunu kapatmayı da öğretiyor.

![Terminalde DoctorDock: komut listesi ve doctordock explain DD005 çıktısı](https://doctordock.iamcanturk.dev/assets/demo.gif)
*Her kural kendini anlatır: `doctordock explain DD005` saldırı senaryosunu, çözümü ve kaynakları gösterir.*

## Kurallar: güvenlik, yapılandırma, kaynak, temizlik

DoctorDock şu an güvenlik, yapılandırma, kaynak ve temizlik başlıkları altında **18 kural** çalıştırıyor. Birkaç öne çıkanı:

- **DD005 (CRITICAL)** — Docker socket bir konteynere mount edilmiş. Bu pratikte host üzerinde root demek.
- **DD002 (CRITICAL)** — Ayrıcalıklı (privileged) konteyner.
- **DD001 (HIGH)** — Konteyner root olarak çalışıyor.
- **DD004 (HIGH)** — Hassas host yolu mount edilmiş (`/`, `/etc`, `~/.ssh`, `~/.aws` gibi).
- **DD006 (MEDIUM)** — Veritabanı portu `0.0.0.0` üzerinde yayınlanmış.
- **DD013 (MEDIUM)** — Konteyner restart döngüsünde.
- **DD007 (LOW)** — Healthcheck yok.
- **DD015 (INFO)** — Kullanılmayan imaj.

Her biri o `explain` çıktısının arkasında duruyor, yani hiçbiri sizi "sanırım burada bir sorun olabilir" belirsizliğinde bırakmıyor.

## Güvenli temizlik

**Docker temizliği**, bu araçta ayrı bir özenle ele aldığım kısım. Çünkü temizlik komutları insanları haklı olarak korkutur, bir yanlış flag'le aylık verinizi uçurabilirsiniz.

DoctorDock bu yüzden varsayılan olarak hiçbir şey silmez:

```bash
doctordock cleanup            # kuru çalışma: sadece ne silineceğini gösterir
doctordock cleanup --apply    # gerçekten uygular
```

`cleanup` komutu `--apply` vermediğiniz sürece bir **kuru çalışmadır (dry run)**. Sadece neyi geri kazanabileceğinizi gösterir, hiçbir şeye dokunmaz.

Daha da önemlisi: `--volumes` dışında hiçbir flag bir volume'u seçemez, `--all` bile. Bunun sebebi basit. Bir imajı yeniden çekebilir, bir ağı yeniden oluşturabilir, durmuş bir konteyneri yeniden ayağa kaldırabilirsiniz. Ama bir volume'daki veri gittiğinde geri gelmez. Bu yüzden volume'lar ancak siz açıkça ve tek başına istediğinizde seçime dahil olur. Kazara hiçbir şey silinmez.

## CI'da kullanım

DoctorDock'u pipeline'ınıza koymak isterseniz çıkış kodları isteğe bağlı olarak devreye giriyor:

```bash
doctordock scan --fail-on high
```

Temizse `0`, HIGH bulgu varsa `2`, CRITICAL varsa `3`, aracın kendisi çökerse `10` döner. Böylece ciddiyet eşiğini kendiniz belirlersiniz. Bir bulgu her göründüğünde build'i kırmak zorunda değilsiniz.

## Yol haritası

Şu anki sürüm (v0.1) çalışan bir Docker ortamının teşhisine odaklanıyor. Sonraki sürümlerde iki yöne genişletmeyi planlıyorum:

- **Dockerfile ve Compose analizi** — sorunları ortam ayağa kalkmadan, yazım aşamasında yakalamak.
- **SARIF çıktısı** — bulguları GitHub Code Scanning ve benzeri araçların anladığı standart formatta vermek.

Her adımda aynı ilke geçerli olacak: çevrimdışı, deterministik, gizliliğe saygılı.

## Kapanış

DoctorDock, Docker ortamınızın aylar içinde biriktirdiği görünmez riskleri saniyeden kısa sürede, hiçbir veriyi dışarı sızdırmadan önünüze koyan küçük bir araç. Bir doktor gibi: bakar, teşhis koyar, ne yapmanız gerektiğini anlatır ve kararı size bırakır.

Denemek isterseniz:

- GitHub'da yıldız bırakın: [github.com/iamcanturk/DoctorDock](https://github.com/iamcanturk/DoctorDock)
- Detaylar ve indirme için: [doctordock.iamcanturk.dev](https://doctordock.iamcanturk.dev)
- Yeni sürümleri takip etmek için X'te [@iamcanturk](https://x.com/iamcanturk)

Bir tarama çalıştırın ve makinenizin gerçekte hangi puanı aldığını görün. Muhtemelen sandığınızdan düşük çıkacak, ve ilk kez tam olarak ne yapacağınızı bileceksiniz.
