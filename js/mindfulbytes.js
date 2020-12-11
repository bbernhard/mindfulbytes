window.addEventListener('load', function() {
    new Typewriter('#typewriter', {
        strings: ['Get creative.', 'Use mindfulbytes in your own projects.', 'Create your own picture frame.'],
        autoStart: true,
        loop: true,
    });

    AOS.init({
        once: true
    });
})