import { createRouter, createWebHistory } from 'vue-router';
import Base64Tools from '../components/Base64Tools.vue';
import URLTools from '../components/URLTools.vue';
import DateTimeTools from '../components/DateTimeTools.vue';
import JsonTools from '../components/JsonTools.vue';
import Sha1Tools from '../components/Sha1Tools.vue';
import MD5Tools from '../components/MD5Tools.vue';
import PortsTools from '../components/PortsTools.vue';
import ImageTools from '../components/ImageTools.vue';
import Home from '../views/Home.vue'

const routes = [
    { path: '/', name: 'Home', component: Home },
    { path: '/base64', name: 'Base64Tools', component: Base64Tools },
    { path: '/url', name: 'URLTools', component: URLTools },
    { path: '/datetime', name: 'DateTimeTools', component: DateTimeTools },
    { path: '/json', name: 'JsonTools', component: JsonTools },
    { path: '/sha1', name: 'Sha1Tools', component: Sha1Tools },
    { path: '/md5', name: 'MD5Tools', component: MD5Tools },
    { path: '/qrcode', name: 'QrCode', component: () => import('../views/QrCode.vue') },
    { path: '/ports', name: 'PortsTools', component: PortsTools },
    { path: '/image', name: 'ImageTools', component: ImageTools },

];

const router = createRouter({
    history: createWebHistory(),
    routes
});

export default router;