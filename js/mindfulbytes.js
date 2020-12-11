window.addEventListener('load', function() {
    const typewriter = new Typewriter('#typewriter', {
        loop: true,
    });

    typewriter.typeString("Get creative.")
        .pauseFor(2500)
        .deleteAll()
        .typeString("Use mindfulbytes in your own projects.")
        .pauseFor(2500)
        .deleteAll()
        .callFunction(() => {
            if ($("#picture-frame-img").hasClass("is-hidden")) {
                $("#picture-frame-img").removeClass("is-hidden");
                $("#signal-notification-img").addClass("is-hidden");
                $("#tablet-img").addClass("is-hidden");
                $("#picture-frame-img").fadeIn(1000);
            }
        })
        .typeString("Create your own picture frame.")
        .pauseFor(2500)
        .deleteAll()
        .callFunction(() => {
            if ($("#signal-notification-img").hasClass("is-hidden")) {
                $("#signal-notification-img").removeClass("is-hidden");
                $("#picture-frame-img").addClass("is-hidden");
                $("#tablet-img").addClass("is-hidden");
                $("#signal-notification-img").fadeIn(1000);
            }
        })
        .typeString("Get your mindful reminder via Signal.")
        .pauseFor(2500)
        .deleteAll()
        .callFunction(() => {
            if ($("#tablet-img").hasClass("is-hidden")) {
                $("#tablet-img").removeClass("is-hidden");
                $("#picture-frame-img").addClass("is-hidden");
                $("#signal-notification-img").addClass("is-hidden");
                $("#tablet-img").fadeIn(1000);
            }
        })
        .typeString("Create a mindful screensaver on your tablet.")
        .pauseFor(2500)
        .start();

    AOS.init({
        once: true
    });
})