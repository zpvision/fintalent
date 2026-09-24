package main

import "strings"

const yandexMetrikaCounter = `<!-- Yandex.Metrika counter -->
<script type="text/javascript">
    (function(m,e,t,r,i,k,a){
        m[i]=m[i]||function(){(m[i].a=m[i].a||[]).push(arguments)};
        m[i].l=1*new Date();
        for (var j = 0; j < document.scripts.length; j++) {if (document.scripts[j].src === r) { return; }}
        k=e.createElement(t),a=e.getElementsByTagName(t)[0],k.async=1,k.src=r,a.parentNode.insertBefore(k,a)
    })(window, document,'script','https://mc.yandex.ru/metrika/tag.js?id=113008460', 'ym');

    ym(113008460, 'init', {ssr:true, webvisor:true, clickmap:true, ecommerce:"dataLayer", referrer: document.referrer, url: location.href, accurateTrackBounce:true, trackLinks:true});
</script>
<!-- /Yandex.Metrika counter -->`

const yandexMetrikaNoScript = `<noscript><div><img src="https://mc.yandex.ru/watch/113008460" style="position:absolute; left:-9999px;" alt="" /></div></noscript>`

func injectYandexMetrika(content []byte) []byte {
	page := string(content)
	if strings.Contains(page, "mc.yandex.ru/metrika/tag.js?id=113008460") {
		return content
	}
	if strings.Contains(page, "</head>") {
		page = strings.Replace(page, "</head>", yandexMetrikaCounter+"\n</head>", 1)
	}
	if strings.Contains(page, "</body>") {
		page = strings.Replace(page, "</body>", yandexMetrikaNoScript+"\n</body>", 1)
	}
	return []byte(page)
}
